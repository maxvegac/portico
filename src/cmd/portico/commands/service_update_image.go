package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
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

			// Get app-name and service-name from parent command or auto-detect
			appName, serviceName, err := getAppAndServiceFromArgs(cmd)
			if err != nil || appName == "" || serviceName == "" {
				fmt.Println("Error: app-name and service-name are required")
				fmt.Println()
				fmt.Println("Usage:")
				fmt.Println("  portico service [app-name] [service-name] image [image-name]")
				fmt.Println()
				fmt.Println("Examples:")
				fmt.Println("  portico service my-app web image myregistry.com/my-app:v1.0.0")
				fmt.Println("  portico service mail-worker worker image ghcr.io/user/worker:latest")
				fmt.Println()
				fmt.Println("Note: Service-name can be auto-detected if the app has only one service.")
				return
			}

			if err := util.ValidateServiceName(serviceName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := loadOrCreateApp(cfg, appManager, appName)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			isFirstService := len(appConfig.Services) == 0
			_, created := setServiceImage(appConfig, serviceName, imageName)
			if created {
				fmt.Printf("Creating service %s in app %s...\n", serviceName, appName)
			} else {
				fmt.Printf("Updating service %s in app %s...\n", serviceName, appName)
			}

			// HTTP configuration
			switch {
			case noHTTPPort:
				// Worker: if it was the HTTP service, disable HTTP for the app
				if appConfig.HTTPServiceName() == serviceName {
					appConfig.Port = 0
					appConfig.HttpService = ""
				}
			case port > 0:
				// Explicit port: this service receives HTTP traffic on that port
				appConfig.Port = port
				appConfig.HttpService = serviceName
			case created && (isFirstService || serviceName == "web"):
				// First (or "web") service: receives HTTP traffic
				if appConfig.Port == 0 {
					appConfig.Port = app.DefaultHTTPPort
				}
				appConfig.HttpService = serviceName
			}

			pullImage(imageName)

			if err := applyApp(cfg, appManager, appConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			if created {
				fmt.Printf("✅ Service %s created in app %s with image %s\n", serviceName, appName, imageName)
			} else {
				fmt.Printf("✅ Service %s in app %s updated to image %s\n", serviceName, appName, imageName)
			}
			if appConfig.Port > 0 && appConfig.HTTPServiceName() == serviceName {
				fmt.Printf("HTTP: %s -> %s:%d\n", appURL(appConfig), serviceName, appConfig.Port)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Internal container port that receives HTTP traffic (makes this the app's HTTP service)")
	cmd.Flags().BoolVar(&noHTTPPort, "no-http-port", false, "Create a background worker without HTTP port")

	return cmd
}
