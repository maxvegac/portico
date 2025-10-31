package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewAppsDownCmd baja los servicios (docker compose down) de una app
func NewAppsDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [app-name]",
		Short: "Stop application services",
		Long:  "Stop services for the given application using Docker Compose (equivalent to 'docker compose down').",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Load app config
			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app config: %v\n", err)
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)

			// Detect manual changes to docker-compose.yml
			hasManualChanges, err := dm.DetectManualChanges(appDir)
			if err != nil {
				fmt.Printf("Warning: Could not check for manual changes: %v\n", err)
			} else if hasManualChanges {
				fmt.Println("⚠️  Warning: docker-compose.yml appears to have been manually modified.")
				fmt.Println("Portico will regenerate it, preserving your custom fields.")
				fmt.Print("Continue? (y/N): ")
				reader := bufio.NewReader(os.Stdin)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(response)
				if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
					fmt.Println("Cancelled.")
					return
				}
			}

			// Prepare services and metadata
			var dockerServices []docker.Service
			for _, service := range appConfig.Services {
				dockerServices = append(dockerServices, docker.Service{
					Name:        service.Name,
					Image:       service.Image,
					Port:        service.Port,
					ExtraPorts:  service.ExtraPorts,
					Environment: service.Environment,
					Volumes:     service.Volumes,
					Secrets:     service.Secrets,
					DependsOn:   service.DependsOn,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: appConfig.Domain,
				Port:   appConfig.Port,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Stop services
			if err := dm.StopApp(appDir); err != nil {
				fmt.Printf("Error stopping services: %v\n", err)
				return
			}

			fmt.Printf("Services for %s are down.\n", appName)
		},
	}
}
