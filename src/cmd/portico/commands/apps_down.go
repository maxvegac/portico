package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

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

			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)
			if err := dm.StopApp(appDir); err != nil {
				fmt.Printf("Error stopping services: %v\n", err)
				return
			}

			fmt.Printf("Services for %s are down.\n", appName)
		},
	}
}
