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

// NewAppsDestroyCmd creates the apps destroy command
func NewAppsDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy [app-name]",
		Short: "Destroy an application",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			fmt.Printf("Destroying application: %s\n", appName)

			// Load config
			config, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(config.AppsDir)

			// Check if app exists
			appDir := filepath.Join(config.AppsDir, appName)
			if _, err := appManager.LoadApp(appName); err != nil {
				fmt.Printf("Error: Application '%s' does not exist\n", appName)
				return
			}

			// Create docker manager
			dockerManager := docker.NewManager(config.Registry.URL)

			// Stop the application if it's running
			composeFile := filepath.Join(appDir, "docker-compose.yml")
			if _, err := os.Stat(composeFile); err == nil {
				fmt.Printf("Stopping application containers...\n")
				if err := dockerManager.StopApp(appDir); err != nil {
					fmt.Printf("Warning: Error stopping application containers: %v\n", err)
					fmt.Printf("Continuing with application deletion...\n")
				} else {
					fmt.Printf("Application containers stopped successfully\n")
				}
			}

			// Delete the application
			if err := appManager.DeleteApp(appName); err != nil {
				fmt.Printf("Error deleting application: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(config.ProxyDir)
			if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
				fmt.Printf("Warning: Error updating Caddyfile: %v\n", err)
			}

			fmt.Printf("Application %s destroyed successfully!\n", appName)
		},
	}
}
