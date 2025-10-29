package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewAppsCreateCmd creates the apps create command
func NewAppsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [app-name]",
		Short: "Create a new application",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			fmt.Printf("Creating application: %s\n", appName)

			// Load config
			config, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(config.AppsDir, config.TemplatesDir)

			// Create the app
			if err := appManager.CreateApp(appName); err != nil {
				fmt.Printf("Error creating app: %v\n", err)
				return
			}

			// Update Caddyfile
			proxyManager := proxy.NewCaddyManager(config.ProxyDir, config.TemplatesDir)
			if err := proxyManager.UpdateCaddyfile(config.AppsDir); err != nil {
				fmt.Printf("Error updating Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("Application %s created successfully!\n", appName)
		},
	}
}
