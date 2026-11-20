package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewSetHttpCmd handles both enabling and disabling HTTP/Caddy proxy
func NewSetHttpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http [on|off|service-name]",
		Short: "Enable or disable HTTP/Caddy proxy",
		Long: `Enable or disable HTTP/Caddy proxy for an application.

  http off          - Disable HTTP/Caddy proxy (convert to background worker)
  http on           - Enable HTTP/Caddy proxy (uses "web" or the only service)
  http <service>    - Enable HTTP/Caddy proxy using specified service`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			appName, _ := getAppNameFromSetArgs(cmd)
			if appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http [on|off|service-name]")
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

			action := "on"
			serviceName := ""
			if len(args) == 1 {
				switch args[0] {
				case "on", "off":
					action = args[0]
				default:
					serviceName = args[0]
				}
			}

			if action == "off" {
				if a.Port > 0 {
					a.DisabledHTTPPort = a.Port
				}
				a.Port = 0
				a.HttpService = ""
				if err := am.SaveApp(a); err != nil {
					fmt.Printf("Error saving app: %v\n", err)
					return
				}
				if err := syncProxy(cfg, am, a); err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				fmt.Printf("HTTP/Caddy proxy disabled for %s (app is now a background worker)\n", appName)
				return
			}

			if len(a.Services) == 0 {
				fmt.Printf("Error: no services found in app %s\n", appName)
				return
			}

			// Determine which service receives HTTP traffic
			switch {
			case serviceName != "":
				if a.FindService(serviceName) == nil {
					fmt.Printf("Error: service '%s' not found in app %s\n", serviceName, appName)
					printServices(a)
					return
				}
			case a.HttpService != "" && a.FindService(a.HttpService) != nil:
				serviceName = a.HttpService
			case len(a.Services) == 1:
				serviceName = a.Services[0].Name
			case a.FindService("web") != nil:
				serviceName = "web"
			default:
				fmt.Printf("Error: app %s has multiple services. Please specify which service to use:\n", appName)
				printServices(a)
				fmt.Printf("\nUsage: portico set %s http <service-name>\n", appName)
				return
			}

			a.HttpService = serviceName
			if a.Port == 0 && a.DisabledHTTPPort > 0 {
				a.Port = a.DisabledHTTPPort
			}
			if a.Port == 0 {
				a.Port = app.DefaultHTTPPort
				fmt.Printf("Using default internal HTTP port %d (change with 'portico set %s http-port <port>')\n", a.Port, appName)
			}

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Create/update Caddyfile (with prompt for manual changes)
			if err := am.CreateDefaultCaddyfileWithPrompt(appName, true); err != nil {
				if errors.Is(err, app.ErrCancelled) {
					return
				}
				fmt.Printf("Error: could not create Caddyfile: %v\n", err)
				return
			}
			if err := syncProxy(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("HTTP/Caddy proxy enabled for %s using service '%s' (port: %d)\n", appName, serviceName, a.Port)
		},
	}
}
