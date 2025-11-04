package commands

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewServiceScaleCmd sets the number of replicas for a service
func NewServiceScaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale [number]",
		Short: "Set the number of instances (replicas) for a service",
		Long: `Set the number of instances (replicas) for a service within an application.
		
This uses Docker Compose's --scale feature to run multiple instances of the same service.
The load will be distributed across all instances by Caddy (for the main service) or Docker's internal load balancing.

Examples:
  # Scale 'web' service to 3 instances
  portico service my-app web scale 3
  
  # Scale 'api' service to 5 instances
  portico service my-app api scale 5
  
  # Scale back down to 1 instance
  portico service my-app web scale 1`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name and service-name from parent command
			appName, serviceName, err := getAppAndServiceFromArgs(cmd)
			if err != nil || appName == "" || serviceName == "" {
				fmt.Println("Error: app-name and service-name are required")
				fmt.Println("Usage: portico service [app-name] [service-name] scale [number]")
				return
			}

			replicas, err := strconv.Atoi(args[0])
			if err != nil || replicas < 1 {
				fmt.Printf("Error: invalid number of replicas: %s (must be at least 1)\n", args[0])
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Find and update the service
			found := false
			for i := range appConfig.Services {
				if appConfig.Services[i].Name == serviceName {
					appConfig.Services[i].Replicas = replicas
					found = true
					break
				}
			}

			if !found {
				fmt.Printf("Error: service %s not found in app %s\n", serviceName, appName)
				return
			}

			// Save app configuration
			if err := appManager.SaveApp(appConfig); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Generate docker-compose.yml
			dockerManager := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)

			var dockerServices []docker.Service
			for _, svc := range appConfig.Services {
				svcReplicas := svc.Replicas
				if svcReplicas == 0 {
					svcReplicas = 1 // Default to 1 if not specified
				}
				dockerServices = append(dockerServices, docker.Service{
					Name:        svc.Name,
					Image:       svc.Image,
					Port:        svc.Port,
					ExtraPorts:  svc.ExtraPorts,
					Environment: svc.Environment,
					Volumes:     svc.Volumes,
					Secrets:     svc.Secrets,
					DependsOn:   svc.DependsOn,
					Replicas:    svcReplicas,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: appConfig.Domain,
				Port:   appConfig.Port,
			}

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Deploy with scale
			if err := dockerManager.DeployApp(appDir, dockerServices); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			// Update Caddyfile (in case it's the main service)
			proxyManager := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("✅ Service %s in app %s scaled to %d instance(s)\n", serviceName, appName, replicas)
		},
	}

	return cmd
}
