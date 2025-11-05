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
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Portico by extracting embedded static files",
		Long: `Extract all embedded static files (Caddyfile, config.yml, docker-compose.yml, index.html, addon definitions) 
to the filesystem. This is typically called during installation.

Files are extracted to their correct locations:
  - /home/portico/templates/*.tmpl (customizable templates)
  - /home/portico/templates/Caddyfile (reference copy)
  - /home/portico/www/index.html
  - /home/portico/config.yml
  - /home/portico/reverse-proxy/docker-compose.yml
  - /home/portico/reverse-proxy/Caddyfile
  - /home/portico/addons/definitions/*.yml

Templates can be customized by editing files in /home/portico/templates/`,
		Args: cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Extract templates to templates directory (for user customization)
			templatesDir := cfg.TemplatesDir
			templateFiles := []string{
				"caddy-app.tmpl",
				"docker-compose.tmpl",
				"app.yml.tmpl",
			}

			for _, templateFile := range templateFiles {
				templatePath := filepath.Join(templatesDir, templateFile)
				if err := embed.ExtractTemplate(templateFile, templatePath); err != nil {
					// Not all templates might exist, so we just warn
					fmt.Printf("Warning: could not extract %s template: %v\n", templateFile, err)
				}
			}

			// Extract Caddyfile to templates directory (for reference)
			caddyfilePath := filepath.Join(cfg.PorticoHome, "templates", "Caddyfile")
			if err := embed.ExtractStaticFile("static/reverse-proxy/Caddyfile", caddyfilePath); err != nil {
				fmt.Printf("Error extracting Caddyfile: %v\n", err)
				return
			}

			// Extract index.html to www directory
			indexPath := filepath.Join(cfg.PorticoHome, "www", "index.html")
			if err := embed.ExtractStaticFile("static/www/index.html", indexPath); err != nil {
				fmt.Printf("Error extracting index.html: %v\n", err)
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
			if err := embed.ExtractStaticFile("static/reverse-proxy/docker-compose.yml", composePath); err != nil {
				fmt.Printf("Error extracting docker-compose.yml: %v\n", err)
				return
			}

			// Extract Caddyfile to reverse-proxy directory
			reverseProxyCaddyfile := filepath.Join(cfg.ProxyDir, "Caddyfile")
			if err := embed.ExtractStaticFile("static/reverse-proxy/Caddyfile", reverseProxyCaddyfile); err != nil {
				fmt.Printf("Error extracting Caddyfile to reverse-proxy: %v\n", err)
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

			fmt.Printf("✅ Static files extracted successfully\n")
		},
	}

	return cmd
}
