package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/embed"
)

// NewInitCmd creates the init command for extracting static files
func NewInitCmd() *cobra.Command {
	var targetDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Portico by extracting embedded static files",
		Long: `Extract all embedded static files (Caddyfile, config.yml, docker-compose.yml, index.html, addon definitions) 
to the filesystem. This is typically called during installation.

Examples:
  # Initialize to default location (/home/portico)
  portico init
  
  # Initialize to custom location
  portico init --target /custom/path`,
		Args: cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Use target directory if provided, otherwise use portico home
			if targetDir == "" {
				targetDir = cfg.PorticoHome
			}

			// Extract static files
			if err := embed.ExtractStaticFiles(targetDir); err != nil {
				fmt.Printf("Error extracting static files: %v\n", err)
				return
			}

			// Extract config.yml to portico home root
			configPath := filepath.Join(cfg.PorticoHome, "config.yml")
			if err := embed.ExtractStaticFile("static/config.yml", configPath); err != nil {
				fmt.Printf("Error extracting config.yml: %v\n", err)
				return
			}

			// Extract docker-compose.yml to reverse-proxy directory
			composePath := filepath.Join(cfg.ProxyDir, "docker-compose.yml")
			if err := embed.ExtractStaticFile("static/docker-compose.yml", composePath); err != nil {
				fmt.Printf("Error extracting docker-compose.yml: %v\n", err)
				return
			}

			// Extract addon definitions
			addonsDir := filepath.Join(cfg.AddonsDir, "definitions")
			addonTypes := []string{"postgresql", "mysql", "mariadb", "mongodb", "redis", "valkey"}

			for _, addonType := range addonTypes {
				if err := embed.ExtractAddonDefinition(addonType, addonsDir); err != nil {
					// Not all addons might exist, so we just warn
					fmt.Printf("Warning: could not extract %s definition: %v\n", addonType, err)
				}
			}

			fmt.Printf("✅ Static files extracted successfully to %s\n", targetDir)
		},
	}

	cmd.Flags().StringVar(&targetDir, "target", "", "Target directory for static files (default: /home/portico)")

	return cmd
}
