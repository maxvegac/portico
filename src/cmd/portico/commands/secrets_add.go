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

// NewSecretsAddCmd adds a secret file for a service in an app
func NewSecretsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [secret-name] [value]",
		Short: "Add a secret",
		Long:  "Add a secret file for a service in the given app.\n\nExamples:\n  portico secrets my-app add database_password mypassword123\n    Adds database_password secret (uses default service if only one exists)\n\n  portico secrets my-app api add api_key sk-abc123\n    Adds api_key secret for service 'api'",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (secrets)
			appName, err := getAppNameFromSecretsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico secrets [app-name] [service-name] add [secret-name] [value]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromSecretsArgs(cmd)

			secretName := strings.TrimSpace(args[0])
			value := strings.TrimSpace(args[1])

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
					fmt.Println("Usage: portico secrets [app-name] [service-name] add [secret-name] [value]")
					return
				}
			}

			// Find service
			found := false
			serviceIndex := -1
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					serviceIndex = i
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}

			// Check if secret already exists in service
			for _, s := range a.Services[serviceIndex].Secrets {
				if s == secretName {
					fmt.Printf("Secret %s already exists for service %s in %s. Use 'edit' to update it.\n", secretName, serviceName, appName)
					return
				}
			}

			// Create env directory if it doesn't exist
			appDir := filepath.Join(cfg.AppsDir, appName)
			envDir := filepath.Join(appDir, "env")
			if err := os.MkdirAll(envDir, 0o755); err != nil {
				fmt.Printf("Error creating env directory: %v\n", err)
				return
			}

			// Create secret file
			secretPath := filepath.Join(envDir, secretName)
			if err := os.WriteFile(secretPath, []byte(value), 0o600); err != nil {
				fmt.Printf("Error creating secret file: %v\n", err)
				return
			}

			// Fix file ownership if running as root
			_ = util.FixFileOwnership(secretPath)

			// Add secret to service
			if a.Services[serviceIndex].Secrets == nil {
				a.Services[serviceIndex].Secrets = []string{}
			}
			a.Services[serviceIndex].Secrets = append(a.Services[serviceIndex].Secrets, secretName)

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)

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

			// Restart the service to apply new secret
			if err := dm.RestartService(appDir, serviceName); err != nil {
				fmt.Printf("Warning: could not restart service: %v\n", err)
			}

			fmt.Printf("Added secret %s for service %s in %s\n", secretName, serviceName, appName)
		},
	}

	return cmd
}
