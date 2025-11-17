package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewEnvListCmd lists environment variables for services in an app
func NewEnvListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environment variables",
		Long:  "List environment variables for services in an app. If only one service exists, lists that service. Otherwise lists all services.",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (env)
			appName, err := getAppNameFromEnvArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico env [app-name] [service-name] list")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromEnvArgs(cmd)

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

			// Auto-detect service if not specified
			if serviceName == "" {
				if len(a.Services) == 1 {
					serviceName = a.Services[0].Name
				}
			}

			if serviceName != "" {
				// List environment variables for specific service
				found := false
				for _, s := range a.Services {
					if s.Name == serviceName {
						found = true
						fmt.Printf("Environment variables for service %s:\n", serviceName)
						if len(s.Environment) == 0 {
							fmt.Println("  (none)")
						} else {
							for k, v := range s.Environment {
								fmt.Printf("  %s=%s\n", k, v)
							}
						}
						break
					}
				}
				if !found {
					fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				}
			} else {
				// List environment variables for all services
				fmt.Printf("Environment variables for all services in %s:\n\n", appName)
				for _, s := range a.Services {
					fmt.Printf("Service: %s\n", s.Name)
					if len(s.Environment) == 0 {
						fmt.Println("  (none)")
					} else {
						for k, v := range s.Environment {
							fmt.Printf("  %s=%s\n", k, v)
						}
					}
					fmt.Println()
				}
			}
		},
	}

	return cmd
}
