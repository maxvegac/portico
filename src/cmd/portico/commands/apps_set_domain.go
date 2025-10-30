package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewAppsSetDomainCmd cambia el dominio de una app y regenera Caddyfile
func NewAppsSetDomainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-domain [app-name] [domain]",
		Short: "Set application domain",
		Long:  "Update the application's domain in app.yml, regenerate the app Caddyfile, and refresh the reverse proxy.",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			domain := args[1]

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

			a.Domain = domain
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

			fmt.Printf("Domain for %s set to %s\n", appName, domain)
		},
	}
}
