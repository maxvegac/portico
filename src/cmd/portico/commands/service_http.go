package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewServiceHTTPCmd sets the HTTP port for an app (the port Caddy proxies to)
func NewServiceHTTPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http [app-name] [internal-port]",
		Short: "Set application HTTP port",
		Long:  "Set the HTTP port that Caddy will proxy to for the given application. Updates app.yml, regenerates the app Caddyfile, and refreshes the reverse proxy.",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			portStr := args[1]

			port, err := strconv.Atoi(portStr)
			if err != nil || port <= 0 || port > 65535 {
				fmt.Println("Invalid port")
				return
			}

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

			a.Port = port
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

			fmt.Printf("HTTP port for %s set to %d\n", appName, port)
		},
	}
}
