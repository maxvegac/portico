package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAppsListCmd creates the apps list command
func NewAppsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all applications",
		Run: func(_ *cobra.Command, _ []string) {
			// Load config
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Validate apps directory
			if cfg.AppsDir == "" {
				fmt.Printf("Error: apps directory not configured\n")
				return
			}

			// Create app manager
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)

			// List all applications
			apps, err := appManager.ListApps()
			if err != nil {
				fmt.Printf("Error listing applications: %v\n", err)
				return
			}

			// Display results
			if len(apps) == 0 {
				fmt.Println("No applications found.")
				return
			}

			fmt.Printf("Found %d application(s):\n", len(apps))
			for _, appName := range apps {
				fmt.Printf("  - %s\n", appName)
			}
		},
	}
}
