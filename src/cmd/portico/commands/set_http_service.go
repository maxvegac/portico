package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
)

// NewSetHttpServiceCmd sets which service to use for HTTP
func NewSetHttpServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http-service [service-name]",
		Short: "Set which service to use for HTTP (Caddy will proxy to this service)",
		Long:  "Set which service should be used for HTTP. Caddy will proxy requests to this service.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]

			// Get app-name from parent command
			appName, err := getAppNameFromSetArgs(cmd)
			if err != nil || appName == "" {
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

			// Find the service
			var targetService *app.Service
			for i := range a.Services {
				if a.Services[i].Name == serviceName {
					targetService = &a.Services[i]
					break
				}
			}

			if targetService == nil {
				fmt.Printf("Error: service '%s' not found in app %s\n", serviceName, appName)
				fmt.Println("Available services:")
				for _, s := range a.Services {
					fmt.Printf("  - %s\n", s.Name)
				}
				return
			}

			// Set HTTP port - use service port if configured, otherwise default to 8000
			// This port is internal to the container, used by Caddy for reverse proxy
			// It doesn't need to be exposed to the host
			httpPort := targetService.Port
			if httpPort == 0 {
				httpPort = 8000
				fmt.Printf("Using default HTTP port 8000 for service '%s' (internal container port)\n", serviceName)
			}

			// Set this service as HTTP service
			a.Port = httpPort

			// Save app configuration (this regenerates docker-compose.yml with updated services)
			if err := am.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Update Caddyfile (after docker-compose.yml has been updated)
			if err := am.CreateDefaultCaddyfile(appName); err != nil {
				fmt.Printf("Error: could not create Caddyfile: %v\n", err)
				return
			}

			pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
			if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
				fmt.Printf("Error updating proxy Caddyfile: %v\n", err)
				return
			}

			fmt.Printf("HTTP service set to '%s' (internal port: %d) for app %s\n", serviceName, a.Port, appName)
		},
	}
}
