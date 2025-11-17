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

// NewPortsDeleteCmd deletes a port mapping for a service in an app
func NewPortsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [external:internal|http]",
		Short: "Delete a service port mapping or remove HTTP port",
		Long:  "Delete a service port mapping in the given app (default service: auto-detected), or use 'http' to remove the HTTP port (disables Caddy proxy for this app).",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (ports)
			appName, err := getAppNameFromPortsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico ports [app-name] [service-name] delete [external:internal|http]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromPortsArgs(cmd)

			mapping := args[0]

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

			// Special case: remove HTTP port (set to 0 to disable Caddy proxy)
			if mapping == "http" {
				a.Port = 0
				if err := am.SaveApp(a); err != nil {
					fmt.Printf("Error saving app: %v\n", err)
					return
				}
				// Remove app Caddyfile since there's no HTTP port
				appDir := filepath.Join(cfg.AppsDir, appName)
				caddyfilePath := filepath.Join(appDir, "Caddyfile")
				if err := os.Remove(caddyfilePath); err != nil && !os.IsNotExist(err) {
					fmt.Printf("Warning: could not remove app Caddyfile: %v\n", err)
				}
				pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
				if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
					fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
					return
				}
				fmt.Printf("HTTP port removed for %s (Caddy proxy disabled)\n", appName)
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
					fmt.Println("Usage: portico ports [app-name] [service-name] delete [external:internal|http]")
					return
				}
			}

			found := false
			removed := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true
					filtered := make([]string, 0, len(a.Services[i].ExtraPorts))
					for _, m := range a.Services[i].ExtraPorts {
						if m == mapping {
							removed = true
							continue
						}
						filtered = append(filtered, m)
					}
					a.Services[i].ExtraPorts = filtered
					break
				}
			}
			if !found {
				fmt.Printf("Service %s not found in app %s\n", serviceName, appName)
				return
			}
			if !removed {
				fmt.Printf("Mapping %s not found\n", mapping)
				return
			}

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

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
			// Get metadata from docker-compose.yml
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

			fmt.Printf("Deleted mapping %s for service %s in %s\n", mapping, serviceName, appName)
		},
	}

	return cmd
}
