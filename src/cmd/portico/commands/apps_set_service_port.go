package commands

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewAppsSetServicePortCmd sets the port of a specific service and regenerates docker-compose
func NewAppsSetServicePortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "service [app-name] [service-name] [port]",
		Short: "Set a service port and redeploy",
		Long:  "Update the port for a specific service in app.yml, regenerate docker-compose.yml and re-run 'docker compose up -d'.",
		Args:  cobra.ExactArgs(3),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			serviceName := args[1]
			portStr := args[2]

			port, err := strconv.Atoi(portStr)
			if err != nil || port <= 0 || port > 65535 {
				fmt.Println("Invalid port")
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

			found := false
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					a.Services[i].Port = port
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

			// Regenerate docker-compose and redeploy
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

			fmt.Printf("Port for service %s in %s set to %d\n", serviceName, appName, port)
		},
	}
}
