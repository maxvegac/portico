package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAppsDeployCmd creates the apps deploy command
func NewAppsDeployCmd() *cobra.Command {
	var sourcePath string
	var dockerfile string
	var imageName string
	var buildArgs []string
	var service string

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

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			if err := util.ValidateAppName(appName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Build Docker image
			fmt.Printf("Building Docker image: %s\n", imageName)
			fmt.Printf("Source: %s\n", absSourcePath)
			fmt.Printf("Dockerfile: %s\n", dockerfilePath)
			if err := buildImage(imageName, dockerfilePath, absSourcePath, buildArgs); err != nil {
				fmt.Printf("Error building Docker image: %v\n", err)
				return
			}
			fmt.Printf("✅ Docker image built successfully: %s\n", imageName)

			// Load (or create) the app and point its HTTP service at the new image
			appConfig, err := loadOrCreateApp(cfg, appManager, appName)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			serviceName, _ := setServiceImage(appConfig, service, imageName)

			if err := applyApp(cfg, appManager, appConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("✅ Application %s deployed successfully!\n", appName)
			fmt.Printf("Service: %s, Image: %s\n", serviceName, imageName)
			if url := appURL(appConfig); url != "" {
				fmt.Printf("URL: %s\n", url)
			}
		},
	}

	cmd.Flags().StringVar(&sourcePath, "from", "", "Source code directory (default: current directory, used only for manual deployments)")
	cmd.Flags().StringVar(&dockerfile, "dockerfile", "Dockerfile", "Dockerfile name or path (default: Dockerfile)")
	cmd.Flags().StringVar(&imageName, "image", "", "Docker image name (default: portico-<app-name>:latest)")
	cmd.Flags().StringVar(&service, "service", "", "Service to update (default: the HTTP service, or 'web')")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", []string{}, "Build arguments for docker build (can be specified multiple times)")

	return cmd
}
