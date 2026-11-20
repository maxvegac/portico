package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewPortsDeleteCmd deletes a port mapping for a service in an app
func NewPortsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [external:internal]",
		Short: "Delete a service port mapping",
		Long:  "Delete a service port mapping (exposed port) in the given app.\n\nNote: To disable HTTP/Caddy proxy, use 'portico set <app-name> http off'",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (ports)
			appName, err := getAppNameFromPortsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico ports [app-name] [service-name] delete [external:internal]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromPortsArgs(cmd)

			mapping := args[0]

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
					var serviceNames []string
					for _, s := range a.Services {
						serviceNames = append(serviceNames, s.Name)
					}
					fmt.Printf("Error: app %s has %d services. Please specify service name\n", appName, len(a.Services))
					fmt.Printf("Available services: %v\n", serviceNames)
					fmt.Println("Usage: portico ports [app-name] [service-name] delete [external:internal]")
					return
				}
			}

			found := false
			removed := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					filtered := make([]string, 0, len(a.Services[i].ExtraPorts))
					for _, m := range a.Services[i].ExtraPorts {
						if m == mapping {
							removed = true
							continue
						}
						filtered = append(filtered, m)
					}
					a.Services[i].ExtraPorts = filtered
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}
			if !removed {
				fmt.Printf("Mapping %s not found\n", mapping)
				return
			}

			// Save (regenerates docker-compose.yml), redeploy and refresh the proxy
			if err := applyApp(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("Deleted mapping %s for service %s in %s\n", mapping, serviceName, appName)
		},
	}

	return cmd
}
