package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/proxy"
	"github.com/maxvegac/portico/src/internal/util"
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

			// Extract config.yml to portico home root (never overwrite an existing one:
			// it holds user settings such as external_ip and registry credentials)
			configPath := filepath.Join(cfg.PorticoHome, "config.yml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				if err := embed.ExtractStaticFile("static/config.yml", configPath); err != nil {
					fmt.Printf("Error extracting config.yml: %v\n", err)
					return
				}
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
			for _, addonType := range addonTypeNames {
				if err := embed.ExtractAddonDefinition(addonType, addonsDir); err != nil {
					// Not all addons might exist, so we just warn
					fmt.Printf("Warning: could not extract %s definition: %v\n", addonType, err)
				}
			}

			// Proxy configs of published admin tools (mounted into Caddy)
			proxyAddonsDir := filepath.Join(cfg.ProxyDir, "addons")
			if err := os.MkdirAll(proxyAddonsDir, 0o755); err != nil {
				fmt.Printf("Warning: could not create %s: %v\n", proxyAddonsDir, err)
			}
			_ = util.FixDirOwnership(proxyAddonsDir)

			// Create logs/apps directory if it doesn't exist
			logsAppsDir := filepath.Join(cfg.PorticoHome, "logs", "apps")
			if err := os.MkdirAll(logsAppsDir, 0o755); err != nil {
				fmt.Printf("Warning: could not create logs/apps directory: %v\n", err)
			}

			// Create symbolic link for Docker container logs
			containersLink := filepath.Join(cfg.PorticoHome, "logs", "containers")
			if _, err := os.Lstat(containersLink); os.IsNotExist(err) {
				// Link doesn't exist, create it
				if err := os.Symlink("/var/lib/docker/containers", containersLink); err != nil {
					fmt.Printf("Warning: could not create symbolic link for Docker container logs: %v\n", err)
					fmt.Printf("  You can create it manually with: sudo ln -sf /var/lib/docker/containers %s && sudo chown -h portico:portico %s\n", containersLink, containersLink)
				} else {
					// Fix ownership of the symbolic link
					_ = util.FixFileOwnership(containersLink)
					fmt.Printf("✅ Created symbolic link: %s -> /var/lib/docker/containers\n", containersLink)
					fmt.Printf("  Note: You may need to set ACLs for portico user to access container logs:\n")
					fmt.Printf("  sudo setfacl -R -m u:portico:r-x /var/lib/docker/containers\n")
					fmt.Printf("  sudo setfacl -R -d -m u:portico:r-x /var/lib/docker/containers\n")
				}
			} else {
				// Link exists, fix ownership and check if it's valid
				_ = util.FixFileOwnership(containersLink)
				if _, err := os.Readlink(containersLink); err != nil {
					fmt.Printf("Warning: %s exists but is not a valid symbolic link\n", containersLink)
				}
			}

			// Reload Caddy if it is already running (ignored otherwise)
			pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			_ = pm.ReloadCaddy()

			fmt.Printf("✅ Static files extracted successfully\n")
			fmt.Printf("If the reverse proxy was already running, apply the new configuration with:\n")
			fmt.Printf("  cd %s && docker compose up -d\n", cfg.ProxyDir)
		},
	}

	return cmd
}
