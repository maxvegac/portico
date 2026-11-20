package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewSecretsDeleteCmd deletes a secret file for a service in an app
func NewSecretsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "del [secret-name]",
		Aliases: []string{"delete"},
		Short:   "Delete a secret",
		Long:    "Delete a secret file for a service in the given app.\n\nExamples:\n  portico secrets my-app del database_password\n    Deletes database_password secret (uses default service if only one exists)\n\n  portico secrets my-app api del api_key\n    Deletes api_key secret for service 'api'",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (secrets)
			appName, err := getAppNameFromSecretsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico secrets [app-name] [service-name] del [secret-name]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromSecretsArgs(cmd)

			secretName := strings.TrimSpace(args[0])
			if err := util.ValidateSecretName(secretName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			if secretName == "" {
				fmt.Println("Error: secret-name is required")
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
					fmt.Println("Usage: portico secrets [app-name] [service-name] del [secret-name]")
					return
				}
			}

			// Find service and remove secret
			found := false
			removed := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					// Remove from Secrets list
					filtered := make([]string, 0, len(a.Services[i].Secrets))
					for _, s := range a.Services[i].Secrets {
						if s == secretName {
							removed = true
							continue
						}
						filtered = append(filtered, s)
					}
					a.Services[i].Secrets = filtered
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}
			if !removed {
				fmt.Printf("Secret %s not found for service %s in %s\n", secretName, serviceName, appName)
				return
			}

			// Save (regenerates docker-compose.yml), redeploy and refresh the proxy
			if err := applyApp(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Delete the secret file only if no other service still uses it
			appDir := filepath.Join(cfg.AppsDir, appName)
			stillUsed := false
			for _, s := range a.Services {
				for _, secret := range s.Secrets {
					if secret == secretName {
						stillUsed = true
					}
				}
			}
			if !stillUsed {
				secretPath := filepath.Join(appDir, "env", secretName)
				if err := os.Remove(secretPath); err != nil && !os.IsNotExist(err) {
					fmt.Printf("Warning: could not delete secret file: %v\n", err)
				}
			}

			dm := docker.NewManager(cfg.Registry.URL)

			// Restart the service to apply removed secret
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Deleted secret %s from service %s in %s\n", secretName, serviceName, appName)
		},
	}

	return cmd
}
