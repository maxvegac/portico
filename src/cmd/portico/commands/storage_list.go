package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewStorageListCmd lists volume mounts for services in an app
func NewStorageListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List volume mounts",
		Long:  "List volume mounts for services in an app. If only one service exists, lists that service. Otherwise lists all services.",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (storage)
			appName, err := getAppNameFromStorageArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico storage [app-name] list")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Get service name from flag if provided
			serviceName, _ := cmd.Flags().GetString("name")

			// If only one service and no flag specified, show that service
			if serviceName == "" && len(a.Services) == 1 {
				serviceName = a.Services[0].Name
			}

			if serviceName != "" {
				// List volumes for specific service
				found := false
				for _, s := range a.Services {
					if s.Name == serviceName {
						found = true
						fmt.Printf("Volume mounts for service %s:\n", serviceName)
						if len(s.Volumes) == 0 {
							fmt.Println("  (none)")
						} else {
							for _, v := range s.Volumes {
								fmt.Printf("  - %s\n", v)
							}
						}
						break
					}
				}
				if !found {
					fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				}
			} else {
				// List volumes for all services
				fmt.Printf("Volume mounts for all services in %s:\n\n", appName)
				for _, s := range a.Services {
					fmt.Printf("Service: %s\n", s.Name)
					if len(s.Volumes) == 0 {
						fmt.Println("  (none)")
					} else {
						for _, v := range s.Volumes {
							fmt.Printf("  - %s\n", v)
						}
					}
					fmt.Println()
				}
			}
		},
	}

	cmd.Flags().String("name", "", "service name (optional, required if app has multiple services)")
	return cmd
}
