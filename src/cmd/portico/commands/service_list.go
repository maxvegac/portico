package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewServiceListCmd lists port mappings for a service in an app
func NewServiceListCmd() *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "list [app-name]",
		Short: "List service port mappings",
		Long:  "List the primary and extra port mappings for the selected service in an app.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

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

			if serviceName == "" {
				serviceName = "api"
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

	cmd.Flags().StringVar(&serviceName, "name", "api", "service name (default: api)")
	return cmd
}
