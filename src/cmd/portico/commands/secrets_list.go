package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewSecretsListCmd lists secrets for services in an app
func NewSecretsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List secrets",
		Long:  "List secrets for services in an app. If only one service exists, lists that service. Otherwise lists all services.",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (secrets)
			appName, err := getAppNameFromSecretsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico secrets [app-name] [service-name] list")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromSecretsArgs(cmd)

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

			appDir := filepath.Join(cfg.AppsDir, appName)
			envDir := filepath.Join(appDir, "env")

			if serviceName != "" {
				// List secrets for specific service
				found := false
				for _, s := range a.Services {
					if s.Name == serviceName {
						found = true
						fmt.Printf("Secrets for service %s:\n", serviceName)
						if len(s.Secrets) == 0 {
							fmt.Println("  (none)")
						} else {
							for _, secretName := range s.Secrets {
								secretPath := filepath.Join(envDir, secretName)
								if _, err := os.Stat(secretPath); err == nil {
									fmt.Printf("  ✓ %s (file exists)\n", secretName)
								} else {
									fmt.Printf("  ✗ %s (file missing)\n", secretName)
								}
							}
						}
						break
					}
				}
				if !found {
					fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				}
			} else {
				// List secrets for all services
				fmt.Printf("Secrets for all services in %s:\n\n", appName)
				for _, s := range a.Services {
					fmt.Printf("Service: %s\n", s.Name)
					if len(s.Secrets) == 0 {
						fmt.Println("  (none)")
					} else {
						for _, secretName := range s.Secrets {
							secretPath := filepath.Join(envDir, secretName)
							if _, err := os.Stat(secretPath); err == nil {
								fmt.Printf("  ✓ %s (file exists)\n", secretName)
							} else {
								fmt.Printf("  ✗ %s (file missing)\n", secretName)
							}
						}
					}
					fmt.Println()
				}
			}
		},
	}

	return cmd
}
