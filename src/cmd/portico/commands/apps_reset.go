package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
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
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Create app manager
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)

			// Load app configuration
			appConfig, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app config: %v\n", err)
				return
			}

			// Regenerate docker-compose.yml and Caddyfile, redeploy and reload the proxy
			if err := applyApp(cfg, appManager, appConfig); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("Application %s reset successfully!\n", appName)
		},
	}
}
