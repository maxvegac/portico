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

// NewSetHttpPortCmd sets the HTTP port for an app
func NewSetHttpPortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http-port [port]",
		Short: "Set HTTP port (used by Caddy reverse proxy)",
		Long:  "Set the HTTP port for an application. This port will be used by Caddy for reverse proxying.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			portStr := args[0]

			// Get app-name from parent command
			appName, err := getAppNameFromSetArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http-port <port>")
				return
			}

			port, err := strconv.Atoi(portStr)
			if err != nil || port <= 0 || port > 65535 {
				fmt.Println("Error: invalid port number")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Load docker-compose.yml directly to check http_enabled
			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)
			compose, err := dm.LoadComposeFile(appDir)
			if err != nil {
				fmt.Printf("Error loading docker-compose.yml: %v\n", err)
				return
			}

			// Check if HTTP is enabled
			if compose.XPortico == nil || !compose.XPortico.HttpEnabled {
				fmt.Printf("Error: HTTP is not enabled for app %s. Use 'portico set %s http-service <service-name>' first\n", appName, appName)
				return
			}

			// Load app to get current domain and services
			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Update HTTP port
			a.Port = port

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose.yml
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
				Domain:      a.Domain,
				Port:        port,
				HttpEnabled: true,
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

			fmt.Printf("HTTP port set to %d for app %s\n", port, appName)
		},
	}
}
