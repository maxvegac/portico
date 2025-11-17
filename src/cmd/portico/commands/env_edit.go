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
		Use:   "edit [key] [value]",
		Short: "Edit an environment variable",
		Long:  "Edit an environment variable for a service in the given app.\n\nExamples:\n  portico env my-app edit NODE_ENV production\n    Updates NODE_ENV=production (uses default service if only one exists)\n\n  portico env my-app api edit DATABASE_URL postgres://...\n    Updates DATABASE_URL for service 'api'",
		Args:  cobra.ExactArgs(2),
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

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			var dockerServices []docker.Service
			for _, s := range a.Services {
				replicas := s.Replicas
				if replicas == 0 {
					replicas = 1 // Default to 1 if not specified
				}
				dockerServices = append(dockerServices, docker.Service{
					Name:        s.Name,
					Image:       s.Image,
					Port:        s.Port,
					ExtraPorts:  s.ExtraPorts,
					Environment: s.Environment,
					Volumes:     s.Volumes,
					Secrets:     s.Secrets,
					DependsOn:   s.DependsOn,
					Replicas:    replicas,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: a.Domain,
				Port:   a.Port,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}
			if err := dm.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			// Restart the service to apply updated environment variable
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Updated environment variable %s=%s for service %s in %s\n", key, value, serviceName, appName)
		},
	}

	return cmd
}
