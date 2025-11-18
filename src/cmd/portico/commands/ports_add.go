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

// NewPortsAddCmd adds a port mapping for a service in an app
func NewPortsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [internal-port] [external-port]",
		Short: "Expose a service port to the host",
		Long: `Expose a service port to the host for direct access.

By default, services communicate via internal Docker network (DNS). Ports are only
exposed to the host when explicitly added using this command.

This is useful for:
  - Debugging: access services directly without going through Caddy
  - Databases: direct access from external tools
  - Non-HTTP services: APIs, WebSockets, etc.

Note: To configure HTTP port (used by Caddy), use 'portico set <app-name> http-port <port>'

Examples:
  # Expose database port to host
  portico ports my-app db add 5432 5433
    Maps host port 5433 to container port 5432 (access via localhost:5433)

  # Expose API port for debugging (bypassing Caddy)
  portico ports my-app api add 3000 8080
    Access API directly at localhost:8080 (in addition to Caddy proxy)`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (ports)
			appName, err := getAppNameFromPortsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico ports [app-name] [service-name] add [internal-port] [external-port]")
				return
			}

			// Get service-name from args (optional)
			serviceName, _ := getServiceNameFromPortsArgs(cmd)

			internal := strings.TrimSpace(args[0])
			external := strings.TrimSpace(args[1])

			if internal == "" || external == "" {
				fmt.Println("Error: both internal and external ports are required")
				fmt.Println("Usage: portico ports [app-name] [service-name] add [internal-port] [external-port]")
				return
			}

			// Validate internal port
			internalPort, err := strconv.Atoi(internal)
			if err != nil || internalPort <= 0 || internalPort > 65535 {
				fmt.Println("Error: invalid internal port")
				return
			}

			// Validate external port - cannot be 80 or 443 (reserved for Caddy)
			externalPort, err := strconv.Atoi(external)
			if err != nil || externalPort <= 0 || externalPort > 65535 {
				fmt.Println("Error: invalid external port")
				return
			}
			if externalPort == 80 || externalPort == 443 {
				fmt.Println("Error: ports 80 and 443 are reserved for Caddy proxy")
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
					fmt.Println("Usage: portico ports [app-name] [service-name] add [internal-port] [external-port]")
					return
				}
			}

			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					found = true

					// Add extra port mapping (expose to host)
					mapping := external + ":" + internal

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
					fmt.Printf("Exposed port: host port %s -> container port %s for service %s in %s\n", external, internal, serviceName, appName)
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
		},
	}

	return cmd
}
