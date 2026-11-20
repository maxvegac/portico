package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAppsPreserveCmd creates the apps preserve command
func NewAppsPreserveCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "preserve [app-name]",
		Short: "Preserve manual changes to files",
		Long:  "Mark files as preserved to prevent Portico from overwriting manual changes. Updates the hash so Portico recognizes your changes as intentional.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			if err := util.ValidateAppName(appName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)

			// If specific file requested
			if file != "" {
				filePath := filepath.Join(appDir, file)
				fmt.Printf("Preserving file: %s\n", filePath)
				// For now, we'll focus on docker-compose.yml
				// Future: could extend to preserve other files like Caddyfile
				if file != "docker-compose.yml" {
					fmt.Printf("Currently only docker-compose.yml can be preserved\n")
					return
				}
			}

			// Load current compose file
			if err := dm.AcceptManualChanges(appDir); err != nil {
				fmt.Printf("Error preserving docker-compose.yml: %v\n", err)
				return
			}

			fmt.Printf("Manual changes to docker-compose.yml have been preserved.\n")
			fmt.Printf("Portico will maintain your customizations in future regenerations.\n")
		},
	}

	cmd.Flags().StringVar(&file, "file", "docker-compose.yml", "File to preserve (default: docker-compose.yml)")
	return cmd
}
