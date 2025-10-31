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

// NewServiceDeleteCmd deletes a port mapping for a service in an app
func NewServiceDeleteCmd() *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "delete [app-name] [external:internal|http]",
		Short: "Delete a service port mapping or remove HTTP port",
		Long:  "Delete a service port mapping in the given app (default service 'api'), or use 'http' to remove the HTTP port (disables Caddy proxy for this app).",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			mapping := args[1]

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
				dockerServices = append(dockerServices, docker.Service{
					Name:        s.Name,
					Image:       s.Image,
					Port:        s.Port,
					ExtraPorts:  s.ExtraPorts,
					Environment: s.Environment,
					Volumes:     s.Volumes,
					Secrets:     s.Secrets,
					DependsOn:   s.DependsOn,
				})
			}
			// Get metadata from app.yml
			metadata := &docker.PorticoMetadata{
				Domain: a.Domain,
				Port:   a.Port,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}
			if err := dm.DeployApp(appDir); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			fmt.Printf("Deleted mapping %s for service %s in %s\n", mapping, serviceName, appName)
		},
	}

	cmd.Flags().StringVar(&serviceName, "name", "api", "service name (default: api)")
	return cmd
}
