package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewSetHttpPortCmd sets the HTTP port for an app
func NewSetHttpPortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http-port [port]",
		Short: "Set HTTP port (used by Caddy reverse proxy)",
		Long:  "Set the internal container port Caddy proxies HTTP traffic to.\n\nExample:\n  portico set my-app http-port 3000",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appName, _ := getAppNameFromSetArgs(cmd)
			if appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http-port <port>")
				return
			}

			port, err := strconv.Atoi(args[0])
			if err != nil || port <= 0 || port > 65535 {
				fmt.Println("Error: invalid port number")
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

			if a.Port == 0 {
				fmt.Printf("Error: HTTP is not enabled for app %s. Use 'portico set %s http on' first\n", appName, appName)
				return
			}

			a.Port = port
			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}
			if err := syncProxy(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("HTTP port set to %d for app %s\n", port, appName)
		},
	}
}
