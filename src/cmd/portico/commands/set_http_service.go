package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewSetHttpServiceCmd sets which service to use for HTTP
func NewSetHttpServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http-service [service-name]",
		Short: "Set which service to use for HTTP (Caddy will proxy to this service)",
		Long:  "Set which service should receive HTTP traffic. Caddy will proxy requests to this service.\n\nExample:\n  portico set my-app http-service api",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]

			appName, _ := getAppNameFromSetArgs(cmd)
			if appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico set <app-name> http-service <service-name>")
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

			if a.FindService(serviceName) == nil {
				fmt.Printf("Error: service '%s' not found in app %s\n", serviceName, appName)
				printServices(a)
				return
			}

			a.HttpService = serviceName
			// Keep the configured HTTP port; enable HTTP with the default port if it was off
			if a.Port == 0 && a.DisabledHTTPPort > 0 {
				a.Port = a.DisabledHTTPPort
				fmt.Printf("HTTP was disabled; enabling it on internal port %d\n", a.Port)
			}
			if a.Port == 0 {
				a.Port = app.DefaultHTTPPort
				fmt.Printf("HTTP was disabled; enabling it on internal port %d (change with 'portico set %s http-port <port>')\n", a.Port, appName)
			}

			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}
			if err := syncProxy(cfg, am, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("HTTP service set to '%s' (internal port: %d) for app %s\n", serviceName, a.Port, appName)
		},
	}
}

// printServices lists the services of an app
func printServices(a *app.App) {
	fmt.Println("Available services:")
	for _, s := range a.Services {
		fmt.Printf("  - %s\n", s.Name)
	}
}
