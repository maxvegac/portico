package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewDomainsRemoveCmd removes a domain from an application
func NewDomainsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [domain]",
		Short: "Remove domain from application",
		Long:  "Remove a domain from the application, update app.yml, regenerate the app Caddyfile, and refresh the reverse proxy.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (domains)
			appName, err := getAppNameFromDomainsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico domains [app-name] remove [domain]")
				return
			}
			domain := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			if a.Domain != domain {
				fmt.Printf("Domain %s not found for app %s (current domain: %s)\n", domain, appName, a.Domain)
				return
			}

			// Remove domain (set to empty or default)
			a.Domain = fmt.Sprintf("%s.localhost", appName)
			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			if err := am.CreateDefaultCaddyfile(appName); err != nil {
				fmt.Printf("Error updating app Caddyfile: %v\n", err)
				return
			}

			pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("Domain %s removed from %s\n", domain, appName)
		},
	}
}
