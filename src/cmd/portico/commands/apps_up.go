package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewAppsUpCmd levanta los servicios (docker compose up -d) de una app
func NewAppsUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [app-name]",
		Short: "Start application services",
		Long:  "Start services for the given application using Docker Compose (equivalent to 'docker compose up -d').",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)
			if err := dm.DeployApp(appDir); err != nil {
				fmt.Printf("Error starting services: %v\n", err)
				return
			}

			fmt.Printf("Services for %s are up!\n", appName)
		},
	}
}
