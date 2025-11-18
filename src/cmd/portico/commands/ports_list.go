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

			// Find HTTP service by matching app.Port with service port
			var httpService *app.Service
			if a.Port > 0 {
				for i := range a.Services {
					if a.Services[i].Port == a.Port {
						httpService = &a.Services[i]
						break
					}
				}
			}

			// Show HTTP port and service name if HTTP service exists
			if httpService != nil {
				fmt.Printf("HTTP service: %s (port: %d, used by Caddy)\n", httpService.Name, a.Port)
				fmt.Println()
			} else if a.Port > 0 {
				fmt.Printf("HTTP port configured: %d, but no service found with this port\n", a.Port)
				fmt.Println()
			} else {
				fmt.Println("App type: Background worker (no HTTP port configured)")
				fmt.Println()
			}

			// If no services exist
			if len(a.Services) == 0 {
				fmt.Println("No services found in this app.")
				return
			}

			// Auto-detect service if not specified
			if serviceName == "" {
				if len(a.Services) == 1 {
					serviceName = a.Services[0].Name
				} else {
					// Multiple services - show all instead of assuming "web"
					fmt.Println("Available services:")
					for _, s := range a.Services {
						fmt.Printf("  - %s (port: %d)\n", s.Name, s.Port)
					}
					fmt.Println()
					fmt.Println("To list ports for a specific service, use:")
					fmt.Printf("  portico ports %s <service-name> list\n", appName)
					return
				}
			}

			// Find and display the specified service
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
				fmt.Printf("Error: Service '%s' not found in app '%s'\n", serviceName, appName)
				fmt.Println()
				fmt.Println("Available services:")
				for _, s := range a.Services {
					fmt.Printf("  - %s (port: %d)\n", s.Name, s.Port)
				}
			}
		},
	}

	return cmd
}
