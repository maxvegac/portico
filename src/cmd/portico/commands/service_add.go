package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewServiceAddCmd adds a port mapping for a service in an app
func NewServiceAddCmd() *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "add [app-name] [internal-port] [external-port]",
		Short: "Add a service port mapping",
		Long:  "Add a port mapping for a service in the given app.\n\nArguments order:\n  - internal-port: Port inside the container\n  - external-port: Port on the host (cannot be 80 or 443, reserved for Caddy)\n\nExamples:\n  portico service my-app add 3000 8080\n    Maps host port 8080 to container port 3000 (default service: 'api')\n\n  portico service my-app add 5432 5433 --name database\n    Maps host port 5433 to container port 5432 for service 'database'",
		Args:  cobra.ExactArgs(3),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			internal := strings.TrimSpace(args[1])
			external := strings.TrimSpace(args[2])

			if internal == "" || external == "" {
				fmt.Println("Invalid ports")
				return
			}

			// Validate external port - cannot be 80 or 443 (reserved for Caddy)
			externalPort, err := strconv.Atoi(external)
			if err != nil || externalPort <= 0 || externalPort > 65535 {
				fmt.Println("Invalid external port")
				return
			}
			if externalPort == 80 || externalPort == 443 {
				fmt.Println("Ports 80 and 443 are reserved for Caddy proxy. Use 'service http' to configure HTTP routing.")
				return
			}

			if serviceName == "" {
				serviceName = "api"
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

			mapping := external + ":" + internal

			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					// ensure unique
					exists := false
					for _, m := range a.Services[i].ExtraPorts {
						if m == mapping {
							exists = true
							break
						}
					}
					if exists {
						fmt.Printf("Port mapping %s already exists for service %s in %s\n", mapping, serviceName, appName)
						return
					}
					a.Services[i].ExtraPorts = append(a.Services[i].ExtraPorts, mapping)
					found = true
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

			// regenerate compose and deploy
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

			fmt.Printf("Added port mapping: host port %s -> container port %s for service %s in %s\n", external, internal, serviceName, appName)
		},
	}

	cmd.Flags().StringVar(&serviceName, "name", "api", "service name (default: api)")
	return cmd
}
