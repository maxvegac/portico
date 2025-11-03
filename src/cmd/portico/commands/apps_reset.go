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

// NewAppsResetCmd creates the apps reset command
func NewAppsResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset [app-name]",
		Short: "Reset application to saved configuration",
		Long:  "Reset application by regenerating docker-compose.yml and Caddyfile from saved configuration, then redeploying. Useful after manual changes or to sync state.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			fmt.Printf("Resetting application: %s\n", appName)

			// Load config
			config, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(config.AppsDir, config.TemplatesDir)

			// Load app configuration
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app config: %v\n", err)
				return
			}

			// Create docker manager
			dockerManager := docker.NewManager(config.Registry.URL)

			// Generate docker-compose.yml
			appDir := filepath.Join(config.AppsDir, appName)

			// Convert app.Service to docker.Service
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

			if err := dockerManager.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			// Deploy the application
			if err := dockerManager.DeployApp(appDir); err != nil {
				fmt.Printf("Error redeploying app: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(config.ProxyDir, config.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("Application %s reset successfully!\n", appName)
		},
	}
}
