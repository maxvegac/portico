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

// NewAppsDeployCmd creates the apps deploy command
func NewAppsDeployCmd() *cobra.Command {
	var sourcePath string
	var dockerfile string
	var imageName string
	var buildArgs []string

	cmd := &cobra.Command{
		Use:   "deploy [app-name]",
		Short: "Deploy application from source code",
		Long: `Deploy an application by building a Docker image from source code and deploying it.
		
This command builds a Docker image from the current directory (or specified path) and deploys it.
The Dockerfile should be in the source directory. Used automatically by git push hooks.

Examples:
  # Deploy from current directory (default)
  portico deploy my-app
  
  # Deploy with custom Dockerfile
  portico deploy my-app --dockerfile Dockerfile.prod
  
  # Deploy with build arguments
  portico deploy my-app --build-arg NODE_ENV=production --build-arg VERSION=1.0.0`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Default source path to current directory (where the command is executed)
			if sourcePath == "" {
				sourcePath = "."
			}

			// Resolve absolute path
			absSourcePath, err := filepath.Abs(sourcePath)
			if err != nil {
				fmt.Printf("Error resolving source path: %v\n", err)
				return
			}

			// Check if source directory exists
			if _, err := os.Stat(absSourcePath); os.IsNotExist(err) {
				fmt.Printf("Error: source directory not found: %s\n", absSourcePath)
				return
			}

			// Default Dockerfile name
			if dockerfile == "" {
				dockerfile = "Dockerfile"
			}

			// Check if Dockerfile exists
			dockerfilePath := filepath.Join(absSourcePath, dockerfile)
			if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
				fmt.Printf("Error: Dockerfile not found: %s\n", dockerfilePath)
				return
			}

			// Generate image name if not provided
			if imageName == "" {
				imageName = fmt.Sprintf("portico-%s:latest", appName)
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)

			// Check if app exists, if not create it
			if _, err := appManager.LoadApp(appName); err != nil {
				fmt.Printf("App %s not found. Creating app...\n", appName)
				if err := appManager.CreateAppDirectories(appName); err != nil {
					fmt.Printf("Error creating app directories: %v\n", err)
					return
				}
			}

			// Build Docker image
			fmt.Printf("Building Docker image: %s\n", imageName)
			fmt.Printf("Source: %s\n", absSourcePath)
			fmt.Printf("Dockerfile: %s\n", dockerfilePath)

			buildCmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerfilePath, absSourcePath)

			// Add build arguments
			for _, arg := range buildArgs {
				buildCmd.Args = append(buildCmd.Args, "--build-arg", arg)
			}

			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr

			if err := buildCmd.Run(); err != nil {
				fmt.Printf("Error building Docker image: %v\n", err)
				return
			}

			fmt.Printf("✅ Docker image built successfully: %s\n", imageName)

			// Load or create app configuration
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				// Create default app config
				appConfig = &app.App{
					Name:   appName,
					Domain: fmt.Sprintf("%s.sslip.io", appName),
					Port:   8080,
					Services: []app.Service{
						{
							Name:  "web",
							Image: imageName,
							Port:  3000,
						},
					},
				}
			} else {
				// Update image in existing services
				for i := range appConfig.Services {
					appConfig.Services[i].Image = imageName
				}
			}

			// Generate docker-compose.yml
			dockerManager := docker.NewManager(cfg.Registry.URL)
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

			// Deploy the application
			if err := dockerManager.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("✅ Application %s deployed successfully!\n", appName)
			fmt.Printf("Image: %s\n", imageName)
		},
	}

	cmd.Flags().StringVar(&sourcePath, "from", "", "Source code directory (default: current directory, used only for manual deployments)")
	cmd.Flags().StringVar(&dockerfile, "dockerfile", "Dockerfile", "Dockerfile name or path (default: Dockerfile)")
	cmd.Flags().StringVar(&imageName, "image", "", "Docker image name (default: portico-<app-name>:latest)")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", []string{}, "Build arguments for docker build (can be specified multiple times)")

	return cmd
}
