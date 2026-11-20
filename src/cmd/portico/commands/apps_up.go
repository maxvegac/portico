package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
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
				fmt.Println("(Use 'portico preserve' to keep the file exactly as it is.)")
				if !confirm("Continue?") {
					fmt.Println("Cancelled.")
					return
				}
			}

			if len(appConfig.Services) == 0 {
				fmt.Printf("App %s has no services yet. Deploy one with 'portico deploy %s' or 'portico service %s web image <image>'\n", appName, appName, appName)
				return
			}

			if err := applyApp(cfg, am, appConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("Services for %s are up!\n", appName)
		},
	}
}
