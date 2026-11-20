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

// NewEnvAddCmd adds an environment variable for a service in an app
func NewEnvAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "add [key] [value]",
		Short:              "Add an environment variable",
		Long:               "Add an environment variable for a service in the given app.\n\nExamples:\n  portico env my-app add NODE_ENV production\n    Adds NODE_ENV=production (uses default service if only one exists)\n\n  portico env my-app api add DATABASE_URL postgres://...\n    Adds DATABASE_URL for service 'api'",
		Args:               cobra.ExactArgs(2),
		DisableFlagParsing: true, // values may start with '-'
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (env)
			appName, err := getAppNameFromEnvArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico env [app-name] [service-name] add [key] [value]")
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
					fmt.Println("Usage: portico env [app-name] [service-name] add [key] [value]")
					return
				}
			}

			// Find service and add environment variable
			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					// Initialize Environment map if nil
					if a.Services[i].Environment == nil {
						a.Services[i].Environment = make(map[string]string)
					}
					// Check if key already exists
					if _, exists := a.Services[i].Environment[key]; exists {
						fmt.Printf("Environment variable %s already exists for service %s in %s. Use 'edit' to update it.\n", key, serviceName, appName)
						return
					}
					// Add the environment variable
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

			// Restart the service to apply new environment variable
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Added environment variable %s for service %s in %s\n", key, serviceName, appName)
		},
	}

	return cmd
}
