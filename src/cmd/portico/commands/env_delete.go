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

// NewEnvDeleteCmd deletes an environment variable for a service in an app
func NewEnvDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "del [key]",
		Aliases: []string{"delete"},
		Short:   "Delete an environment variable",
		Long:    "Delete an environment variable for a service in the given app.\n\nExamples:\n  portico env my-app del NODE_ENV\n    Deletes NODE_ENV (uses default service if only one exists)\n\n  portico env my-app api del DATABASE_URL\n    Deletes DATABASE_URL for service 'api'",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (env)
			appName, err := getAppNameFromEnvArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico env [app-name] [service-name] del [key]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromEnvArgs(cmd)

			key := strings.TrimSpace(args[0])

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
					fmt.Println("Usage: portico env [app-name] [service-name] del [key]")
					return
				}
			}

			// Find service and delete environment variable
			found := false
			deleted := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					if a.Services[i].Environment == nil {
						fmt.Printf("Environment variable %s not found for service %s in %s\n", key, serviceName, appName)
						return
					}
					if _, exists := a.Services[i].Environment[key]; exists {
						delete(a.Services[i].Environment, key)
						deleted = true
					}
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}
			if !deleted {
				fmt.Printf("Environment variable %s not found for service %s in %s\n", key, serviceName, appName)
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

			// Restart the service to apply removed environment variable
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Deleted environment variable %s from service %s in %s\n", key, serviceName, appName)
		},
	}

	return cmd
}
