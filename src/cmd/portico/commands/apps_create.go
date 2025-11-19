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

// NewAppsCreateCmd creates the apps create command
func NewAppsCreateCmd() *cobra.Command {
	var withService string
	var image string
	var noHTTPPort bool
	var servicePort int

	cmd := &cobra.Command{
		Use:   "create [app-name]",
		Short: "Create a new application",
		Long: `Create a new application with directory structure and git repository.
		
By default, this command only creates the necessary directories and git repository.
Services are created when you deploy an image using 'portico deploy' or 'portico service ... image'.

You can optionally create a service immediately using --with-service flag.

Examples:
  # Create app (no services created yet)
  portico create my-app

  # Create app with a web service
  portico create my-app --with-service web --image myregistry.com/my-app:v1.0.0

  # Create app with a background worker
  portico create my-app --with-service worker --image myregistry.com/worker:v1.0.0 --no-http-port

  # Deploy will create services automatically
  portico deploy my-app`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appName := args[0]
			fmt.Printf("Creating application: %s\n", appName)

			// Load config
			config, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(config.AppsDir, config.TemplatesDir)

			// Create app directories and secrets
			if err := appManager.CreateAppDirectories(appName); err != nil {
				fmt.Printf("Error creating app directories: %v\n", err)
				return
			}

			// Create git repository for the app
			porticoHome := filepath.Dir(config.AppsDir)
			reposDir := filepath.Join(porticoHome, "repos")

			if err := os.MkdirAll(reposDir, 0o755); err == nil {
				repoDir := filepath.Join(reposDir, appName+".git")

				// Create bare repository
				gitCmd := exec.Command("git", "init", "--bare", repoDir)
				if err := gitCmd.Run(); err == nil {
					// Create post-receive hook that calls portico git-receive
					postReceiveDst := filepath.Join(repoDir, "hooks", "post-receive")
					hookContent := "#!/bin/sh\nexec portico git-receive\n"

					if err := os.WriteFile(postReceiveDst, []byte(hookContent), 0o755); err == nil {
						hostname, _ := os.Hostname()
						if hostname == "" {
							hostname = "portico-server"
						}
						fmt.Printf("Git repository created: portico@%s:%s.git\n", hostname, appName)
					}
				}
			}

			appDir := filepath.Join(config.AppsDir, appName)

			// Create basic app config (always, even without services)
			appHTTPPort := 8080
			if noHTTPPort {
				appHTTPPort = 0
			}

			appConfig := &app.App{
				Name:     appName,
				Domain:   fmt.Sprintf("%s.sslip.io", appName),
				Port:     appHTTPPort,
				Services: []app.Service{},
			}

			// Generate docker-compose.yml with basic structure (even without services)
			dockerManager := docker.NewManager(config.Registry.URL)
			dockerServices := []docker.Service{}

			metadata := &docker.PorticoMetadata{
				Domain: appConfig.Domain,
				Port:   appConfig.Port,
			}

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// If --with-service flag is provided, create the service
			if withService != "" {
				if image == "" {
					fmt.Printf("Error: --image is required when using --with-service\n")
					return
				}

				// Determine service port
				svcPort := 3000
				if noHTTPPort {
					svcPort = 0
					appHTTPPort = 0
				} else if servicePort > 0 {
					svcPort = servicePort
				}

				// Update app config with service
				appConfig.Port = appHTTPPort
				appConfig.Services = []app.Service{
					{
						Name:  withService,
						Image: image,
						Port:  svcPort,
					},
				}

				// Save app configuration
				if err := appManager.SaveApp(appConfig); err != nil {
					fmt.Printf("Error saving app: %v\n", err)
					return
				}

				// Generate docker-compose.yml with service
				dockerServices = []docker.Service{
					{
						Name:        withService,
						Image:       image,
						Port:        svcPort,
						Environment: make(map[string]string),
						Volumes:     []string{},
						Secrets:     []string{},
						DependsOn:   []string{},
						Replicas:    1,
					},
				}

				metadata = &docker.PorticoMetadata{
					Domain: appConfig.Domain,
					Port:   appConfig.Port,
				}

				if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
					fmt.Printf("Error generating docker compose: %v\n", err)
					return
				}

				// Pull the image (if it's from a registry)
				fmt.Printf("Pulling image: %s\n", image)
				pullCmd := exec.Command("docker", "pull", image)
				if err := pullCmd.Run(); err != nil {
					fmt.Printf("Warning: could not pull image (may be local): %v\n", err)
				}

				// Deploy the application
				if err := dockerManager.DeployApp(appDir, dockerServices); err != nil {
					fmt.Printf("Error deploying app: %v\n", err)
					return
				}

				// Update Caddyfile only if there's an HTTP port
				if appHTTPPort > 0 {
					if err := appManager.CreateDefaultCaddyfile(appName); err != nil {
						fmt.Printf("Warning: could not create Caddyfile: %v\n", err)
					}

					proxyManager := proxy.NewCaddyManager(config.ProxyDir, config.TemplatesDir)
					if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
						fmt.Printf("Error updating Caddyfile: %v\n", err)
						return
					}
				}

				fmt.Printf("✅ Application %s created successfully with service %s!\n", appName, withService)
				return
			}

			fmt.Printf("Application %s created successfully!\n", appName)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  To create a web service:\n")
			fmt.Printf("    portico service %s web image [your-image]\n", appName)
			fmt.Println()
			fmt.Printf("  To deploy from source code:\n")
			fmt.Printf("    portico deploy %s\n", appName)
			fmt.Println()
			fmt.Printf("  To create a background worker:\n")
			fmt.Printf("    portico service %s worker image [your-image] --no-http-port\n", appName)
		},
	}

	cmd.Flags().StringVar(&withService, "with-service", "", "Create a service immediately (requires --image)")
	cmd.Flags().StringVar(&image, "image", "", "Docker image for the service (required with --with-service)")
	cmd.Flags().BoolVar(&noHTTPPort, "no-http-port", false, "Create a background worker without HTTP port")
	cmd.Flags().IntVar(&servicePort, "port", 0, "Internal port for the service (default: 3000 for web services)")

	return cmd
}
