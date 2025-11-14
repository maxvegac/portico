package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewServiceUpdateImageCmd updates the Docker image for a service
func NewServiceUpdateImageCmd() *cobra.Command {
	var port int
	var noHTTPPort bool

	cmd := &cobra.Command{
		Use:   "image [image-name]",
		Short: "Update Docker image for service",
		Long: `Update the Docker image for a service and redeploy the application.
		
This is useful when you build images in CI/CD and want to tell Portico to use a new image
without rebuilding locally. The image should already exist in a Docker registry or locally.

If the service doesn't exist, it will be created. For web services, HTTP port will be configured.
For background workers, use --no-http-port flag.

Examples:
  # Update service image (creates service if it doesn't exist)
  portico service my-app web image myregistry.com/my-app:v1.2.3
  
  # Create background worker without HTTP port
  portico service my-app worker image myregistry.com/worker:latest --no-http-port
  
  # From CI/CD (e.g., GitHub Actions)
  portico service my-app web image ghcr.io/user/repo:${{ github.sha }}`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageName := args[0]

			// Get app-name and service-name from parent command
			appName, serviceName, err := getAppAndServiceFromArgs(cmd)
			if err != nil || appName == "" || serviceName == "" {
				fmt.Println("Error: app-name and service-name are required")
				fmt.Println("Usage: portico service [app-name] [service-name] image [image-name]")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := appManager.LoadApp(appName)
			serviceExists := false
			if err != nil {
				// App doesn't exist or has no docker-compose.yml/app.yml
				// Create app directories if needed
				appDir := filepath.Join(cfg.AppsDir, appName)
				if _, err := os.Stat(appDir); os.IsNotExist(err) {
					if err := appManager.CreateAppDirectories(appName); err != nil {
						fmt.Printf("Error creating app directories: %v\n", err)
						return
					}
				}

				// Create new app config with the service
				servicePort := 3000
				appHTTPPort := 8080
				if noHTTPPort {
					servicePort = 0
					appHTTPPort = 0
				} else if port > 0 {
					servicePort = port
				}

				appConfig = &app.App{
					Name:   appName,
					Domain: fmt.Sprintf("%s.localhost", appName),
					Port:   appHTTPPort,
					Services: []app.Service{
						{
							Name:  serviceName,
							Image: imageName,
							Port:  servicePort,
						},
					},
				}
				fmt.Printf("Creating service %s in app %s...\n", serviceName, appName)
			} else {
				// Find and update the service
				found := false
				for i := range appConfig.Services {
					if appConfig.Services[i].Name == serviceName {
						appConfig.Services[i].Image = imageName
						found = true
						serviceExists = true
						break
					}
				}

				if !found {
					// Service doesn't exist, create it
					var servicePort int
					if noHTTPPort {
						servicePort = 0
					} else if port > 0 {
						servicePort = port
					} else if len(appConfig.Services) == 0 || serviceName == "web" {
						// First service or "web" service gets HTTP port by default
						if appConfig.Port == 0 {
							appConfig.Port = 8080
						}
						servicePort = 3000
					} else {
						// Additional service, assume background worker
						servicePort = 0
					}

					newService := app.Service{
						Name:  serviceName,
						Image: imageName,
						Port:  servicePort,
					}

					appConfig.Services = append(appConfig.Services, newService)
					fmt.Printf("Creating service %s in app %s...\n", serviceName, appName)
				} else {
					fmt.Printf("Updating service %s in app %s...\n", serviceName, appName)
				}
			}

			// Save app configuration
			if err := appManager.SaveApp(appConfig); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Generate docker-compose.yml
			dockerManager := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			var dockerServices []docker.Service
			for _, svc := range appConfig.Services {
				replicas := svc.Replicas
				if replicas == 0 {
					replicas = 1 // Default to 1 if not specified
				}
				dockerServices = append(dockerServices, docker.Service{
					Name:        svc.Name,
					Image:       svc.Image,
					Port:        svc.Port,
					ExtraPorts:  svc.ExtraPorts,
					Environment: svc.Environment,
					Volumes:     svc.Volumes,
					Secrets:     svc.Secrets,
					DependsOn:   svc.DependsOn,
					Replicas:    replicas,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: appConfig.Domain,
				Port:   appConfig.Port,
			}

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Pull the new image (if it's from a registry)
			fmt.Printf("Pulling image: %s\n", imageName)
			pullCmd := exec.Command("docker", "pull", imageName)
			if err := pullCmd.Run(); err != nil {
				fmt.Printf("Warning: could not pull image (may be local): %v\n", err)
			}

			// Deploy the application
			if err := dockerManager.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			// Update Caddyfile only if there's an HTTP port
			if appConfig.Port > 0 {
				// Create Caddyfile if it doesn't exist
				appDir := filepath.Join(cfg.AppsDir, appName)
				caddyfilePath := filepath.Join(appDir, "Caddyfile")
				if _, err := os.Stat(caddyfilePath); os.IsNotExist(err) {
					if err := appManager.CreateDefaultCaddyfile(appName); err != nil {
						fmt.Printf("Warning: could not create Caddyfile: %v\n", err)
					}
				}

				// Update proxy Caddyfile
				proxyManager := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
				if err := proxyManager.UpdateCaddyfile(cfg.AppsDir); err != nil {
					fmt.Printf("Error updating Caddyfile: %v\n", err)
					return
				}
			}

			if serviceExists {
				fmt.Printf("✅ Service %s in app %s updated to image %s\n", serviceName, appName, imageName)
			} else {
				fmt.Printf("✅ Service %s created in app %s with image %s\n", serviceName, appName, imageName)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Internal port for the service (default: 3000 for web services, 0 for workers)")
	cmd.Flags().BoolVar(&noHTTPPort, "no-http-port", false, "Create a background worker without HTTP port")

	return cmd
}

// getAppAndServiceFromArgs extracts app-name and service-name from service command arguments
func getAppAndServiceFromArgs(cmd *cobra.Command) (string, string, error) {
	args := os.Args[1:] // Skip program name
	knownCommands := map[string]bool{
		"image": true,
		"scale": true,
	}

	for i, arg := range args {
		if arg == "service" {
			// Next non-flag argument should be app-name
			appName := ""
			serviceName := ""
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if len(args[j]) > 0 && args[j][0] == '-' {
					continue
				}
				// Skip known commands
				if knownCommands[args[j]] {
					continue
				}
				// First non-flag, non-command should be app-name
				if appName == "" {
					appName = args[j]
				} else if serviceName == "" {
					// Second should be service-name
					serviceName = args[j]
					break
				}
			}
			return appName, serviceName, nil
		}
	}
	return "", "", fmt.Errorf("app-name and service-name not found")
}
