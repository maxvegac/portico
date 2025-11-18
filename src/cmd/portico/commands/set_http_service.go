package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewSetHttpServiceCmd sets which service to use for HTTP
func NewSetHttpServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http-service [service-name]",
		Short: "Set which service to use for HTTP (Caddy will proxy to this service)",
		Long:  "Set which service should be used for HTTP. Caddy will proxy requests to this service.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]

			// Get app-name from parent command
			appName, err := getAppNameFromSetArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http-service <service-name>")
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

			// Find the service
			var targetService *app.Service
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					targetService = &a.Services[i]
					break
				}
			}

			if targetService == nil {
				fmt.Printf("Error: service '%s' not found in app %s\n", serviceName, appName)
				fmt.Println("Available services:")
				for _, s := range a.Services {
					fmt.Printf("  - %s (port: %d)\n", s.Name, s.Port)
				}
				return
			}

			if targetService.Port == 0 {
				fmt.Printf("Error: service '%s' has no port configured. Set a port first.\n", serviceName)
				return
			}

			// Set this service as HTTP service
			a.Port = targetService.Port

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose.yml
			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)

			var dockerServices []docker.Service
			for _, s := range a.Services {
				replicas := s.Replicas
				if replicas == 0 {
					replicas = 1
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

			// Update Caddyfile
			if err := am.CreateDefaultCaddyfile(appName); err != nil {
				fmt.Printf("Warning: could not update Caddyfile: %v\n", err)
			}

			pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("HTTP service set to '%s' (port: %d) for app %s\n", serviceName, targetService.Port, appName)
		},
	}
}
