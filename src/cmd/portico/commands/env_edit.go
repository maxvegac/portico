package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewEnvEditCmd edits an environment variable for a service in an app
func NewEnvEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "edit [key] [value]",
		Short:              "Edit an environment variable",
		Long:               "Edit an environment variable for a service in the given app.\n\nExamples:\n  portico env my-app edit NODE_ENV production\n    Updates NODE_ENV=production (uses default service if only one exists)\n\n  portico env my-app api edit DATABASE_URL postgres://...\n    Updates DATABASE_URL for service 'api'",
		Args:               cobra.ExactArgs(2),
		DisableFlagParsing: true, // values may start with '-'
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (env)
			appName, err := getAppNameFromEnvArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico env [app-name] [service-name] edit [key] [value]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromEnvArgs(cmd)

			key := strings.TrimSpace(args[0])
			if !envKeyRe.MatchString(key) {
				fmt.Printf("Error: invalid environment variable name %q\n", key)
				return
			}
			value := strings.TrimSpace(args[1])

			if key == "" {
				fmt.Println("Error: key is required")
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
					fmt.Println("Usage: portico env [app-name] [service-name] edit [key] [value]")
					return
				}
			}

			// Find service and edit environment variable
			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					// Initialize Environment map if nil
					if a.Services[i].Environment == nil {
						a.Services[i].Environment = make(map[string]string)
					}
					// Update the environment variable (edit can also add if it doesn't exist)
					a.Services[i].Environment[key] = value
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}

			// Save (regenerates docker-compose.yml), redeploy and refresh the proxy
			if err := applyApp(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			// Restart the service to apply updated environment variable
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Updated environment variable %s for service %s in %s\n", key, serviceName, appName)
		},
	}

	return cmd
}
