package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewServiceScaleCmd sets the number of replicas for a service
func NewServiceScaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale [number]",
		Short: "Set the number of instances (replicas) for a service",
		Long: `Set the number of instances (replicas) for a service within an application.
		
This uses Docker Compose's --scale feature to run multiple instances of the same service.
The load will be distributed across all instances by Caddy (for the main service) or Docker's internal load balancing.

Examples:
  # Scale 'web' service to 3 instances
  portico service my-app web scale 3
  
  # Scale 'api' service to 5 instances
  portico service my-app api scale 5
  
  # Scale back down to 1 instance
  portico service my-app web scale 1`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name and service-name from parent command or auto-detect
			appName, serviceName, err := getAppAndServiceFromArgs(cmd)
			if err != nil || appName == "" || serviceName == "" {
				fmt.Println("Error: app-name and service-name are required")
				fmt.Println()
				fmt.Println("Usage:")
				fmt.Println("  portico service [app-name] [service-name] scale [number]")
				fmt.Println()
				fmt.Println("Examples:")
				fmt.Println("  portico service my-app web scale 3")
				fmt.Println("  portico service mail-worker worker scale 2")
				fmt.Println()
				fmt.Println("Note: Service-name can be auto-detected if the app has only one service.")
				return
			}

			replicas, err := strconv.Atoi(args[0])
			if err != nil || replicas < 1 {
				fmt.Printf("Error: invalid number of replicas: %s (must be at least 1)\n", args[0])
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Find and update the service
			found := false
			for i := range appConfig.Services {
				if appConfig.Services[i].Name == serviceName {
					appConfig.Services[i].Replicas = replicas
					found = true
					break
				}
			}

			if !found {
				fmt.Printf("Error: service %s not found in app %s\n", serviceName, appName)
				return
			}

			// Save (replicas are persisted in x-portico), deploy with --scale and refresh proxy
			if err := applyApp(cfg, appManager, appConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("✅ Service %s in app %s scaled to %d instance(s)\n", serviceName, appName, replicas)
		},
	}

	return cmd
}
