package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewPortsListCmd lists port mappings for a service in an app
func NewPortsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List service port mappings",
		Long:  "List the primary and extra port mappings for the selected service in an app.",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (ports)
			appName, err := getAppNameFromPortsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico ports [app-name] [service-name] list")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromPortsArgs(cmd)

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
				} else {
					// Use "web" as default (main service)
					serviceName = "web"
				}
			}

			// Show HTTP port (app level)
			fmt.Printf("App HTTP port (Caddy proxy): %d\n", a.Port)
			fmt.Println()

			found := false
			for _, s := range a.Services {
				if s.Name == serviceName {
					found = true
					fmt.Printf("Service: %s\n", serviceName)
					fmt.Printf("Primary port: %d\n", s.Port)
					if len(s.ExtraPorts) == 0 {
						fmt.Println("Extra ports: (none)")
					} else {
						fmt.Println("Extra ports:")
						for _, m := range s.ExtraPorts {
							fmt.Printf("  - %s\n", m)
						}
					}
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
			}
		},
	}

	return cmd
}
