package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewSetHttpCmd handles both enabling and disabling HTTP/Caddy proxy
func NewSetHttpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http [on|off|service-name]",
		Short: "Enable or disable HTTP/Caddy proxy",
		Long: `Enable or disable HTTP/Caddy proxy for an application.

  http off          - Disable HTTP/Caddy proxy (convert to background worker)
  http on           - Enable HTTP/Caddy proxy (uses first service with port, or specify service)
  http <service>    - Enable HTTP/Caddy proxy using specified service`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command
			appName, err := getAppNameFromSetArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http [on|off|service-name]")
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

			// Determine action based on argument
			var action string
			var serviceName string
			if len(args) == 0 {
				// Default to "on" if no argument
				action = "on"
			} else {
				arg := args[0]
				switch arg {
				case "off":
					action = "off"
				case "on":
					action = "on"
				default:
					// Assume it's a service name
					action = "on"
					serviceName = arg
				}
			}

			if action == "off" {
				// Disable HTTP port
				a.Port = 0

				// Save app configuration
				if err := am.SaveApp(a); err != nil {
					fmt.Printf("Error saving app: %v\n", err)
					return
				}

				// Regenerate docker-compose.yml with updated metadata
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
					Domain:      a.Domain,
					Port:        0,
					HttpEnabled: false,
				}

				if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
					fmt.Printf("Error generating docker compose: %v\n", err)
					return
				}

				// Remove app Caddyfile since there's no HTTP port
				caddyfilePath := filepath.Join(appDir, "Caddyfile")
				if err := os.Remove(caddyfilePath); err != nil && !os.IsNotExist(err) {
					fmt.Printf("Warning: could not remove app Caddyfile: %v\n", err)
				}

				// Update main proxy Caddyfile to remove this app's configuration
				pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
				if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
					fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
					return
				}

				fmt.Printf("HTTP/Caddy proxy disabled for %s (app is now a background worker)\n", appName)
				return
			}

			// Enable HTTP (action == "on")
			// Check if HTTP is already enabled
			if a.Port > 0 {
				fmt.Printf("HTTP is already enabled for app %s (port: %d)\n", appName, a.Port)
				return
			}

			// Check if there are any services available
			if len(a.Services) == 0 {
				fmt.Printf("Error: no services found in app %s\n", appName)
				return
			}

			// Determine which service to use
			var targetService *app.Service
			if serviceName != "" {
				// Service name was specified
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
						fmt.Printf("  - %s\n", s.Name)
					}
					return
				}
			} else if len(a.Services) == 1 {
				// Only one service, use it automatically
				targetService = &a.Services[0]
			} else {
				// Multiple services, prefer "web" or require specification
				for i := range a.Services {
					if a.Services[i].Name == "web" {
						targetService = &a.Services[i]
						break
					}
				}
				if targetService == nil {
					fmt.Printf("Error: app %s has multiple services. Please specify which service to use:\n", appName)
					fmt.Println("Available services:")
					for _, s := range a.Services {
						fmt.Printf("  - %s\n", s.Name)
					}
					fmt.Printf("\nUsage: portico set %s http <service-name>\n", appName)
					return
				}
			}

			// Set HTTP port - use service port if configured, otherwise default to 8000
			// This port is internal to the container, used by Caddy for reverse proxy
			httpPort := targetService.Port
			if httpPort == 0 {
				httpPort = 8000
				fmt.Printf("Using default HTTP port 8000 for service '%s' (internal container port)\n", targetService.Name)
			}

			// Enable HTTP port
			a.Port = httpPort

			// Save app configuration
			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose.yml with updated metadata
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
				Domain:      a.Domain,
				Port:        a.Port,
				HttpEnabled: true,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Create/update Caddyfile (with prompt for manual changes)
			if err := am.CreateDefaultCaddyfileWithPrompt(appName, true); err != nil {
				if err.Error() == "cancelled by user" {
					return
				}
				fmt.Printf("Warning: could not create Caddyfile: %v\n", err)
			}

			// Update main proxy Caddyfile
			pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("HTTP/Caddy proxy enabled for %s using service '%s' (port: %d)\n", appName, targetService.Name, a.Port)
		},
	}
}
