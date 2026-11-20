package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAddonAddCmd adds an inline addon (redis/valkey) as a service to an app
func NewAddonAddCmd() *cobra.Command {
	var addonType string
	var version string

	cmd := &cobra.Command{
		Use:   "add [addon-type]",
		Short: "Add inline addon (redis/valkey) as service to app",
		Long:  "Add an inline addon (redis or valkey) as a service within an application.\n\nExample:\n  portico addons my-app add redis --version 7",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (addons)
			appName, err := getAppNameFromAddonsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico addons [app-name] add [addon-type]")
				return
			}
			addonType = args[0]

			if addonType != "redis" && addonType != "valkey" {
				fmt.Printf("Error: %s is not an inline addon. Only redis and valkey are supported.\n", addonType)
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			def, err := am.LoadDefinition(addonType)
			if err != nil {
				fmt.Printf("Error loading addon definition: %v\n", err)
				return
			}

			if def.ServiceMode != "inline" {
				fmt.Printf("Error: %s is not an inline addon\n", addonType)
				return
			}

			// Get version config
			if version == "" {
				version = def.LatestVersion()
			}
			versionConfig, err := def.GetVersionConfig(version)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				fmt.Printf("Available versions: %v\n", def.GetAvailableVersions())
				return
			}

			// Load app
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Check if service already exists
			for _, svc := range a.Services {
				if svc.Name == addonType {
					fmt.Printf("Error: service %s already exists in app %s\n", addonType, appName)
					return
				}
			}

			// Create service from addon definition
			newService := app.Service{
				Name:        addonType,
				Image:       versionConfig.Image,
				Port:        def.DefaultPort,
				Environment: make(map[string]string),
				Volumes:     []string{},
				Secrets:     []string{},
			}

			// Copy environment variables
			for k, v := range versionConfig.Environment {
				newService.Environment[k] = v
			}

			// Add volumes
			for _, vol := range versionConfig.Volumes {
				// Use relative path from app directory
				volPath := fmt.Sprintf("./addons/%s/data:%s", addonType, vol.ContainerPath)
				newService.Volumes = append(newService.Volumes, volPath)
			}

			// Add secrets
			newService.Secrets = versionConfig.Secrets

			// No host ports: other services reach the addon over portico-network
			// using its "<app>-<addon>" alias.

			// Create secrets directory and files first
			appDir := filepath.Join(cfg.AppsDir, appName)
			envDir := filepath.Join(appDir, "env")
			if err := os.MkdirAll(envDir, 0o755); err != nil {
				fmt.Printf("Error creating env directory: %v\n", err)
				return
			}

			var addonPassword, passwordSecret string
			for _, secretName := range versionConfig.Secrets {
				secretPath := filepath.Join(envDir, secretName)
				value, err := generateSecret(secretName)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
				if err := os.WriteFile(secretPath, []byte(value), 0o600); err != nil {
					fmt.Printf("Error creating secret %s: %v\n", secretName, err)
					return
				}
				if strings.Contains(strings.ToLower(secretName), "password") {
					addonPassword = value
					passwordSecret = secretName
				}
			}

			// Add service to app
			a.Services = append(a.Services, newService)

			// Add environment variables to other services for connection
			envPrefix := strings.ToUpper(addonType)
			host := docker.ServiceAlias(appName, addonType)
			for i := range a.Services {
				if a.Services[i].Name != addonType { // Don't add to the addon service itself
					if a.Services[i].Environment == nil {
						a.Services[i].Environment = make(map[string]string)
					}
					a.Services[i].Environment[envPrefix+"_HOST"] = host
					a.Services[i].Environment[envPrefix+"_PORT"] = fmt.Sprintf("%d", def.DefaultPort)
					if addonPassword != "" {
						a.Services[i].Environment[envPrefix+"_PASSWORD"] = addonPassword
						a.Services[i].Environment[envPrefix+"_URL"] = fmt.Sprintf("redis://:%s@%s:%d", addonPassword, host, def.DefaultPort)
					}
				}
			}

			// Create addon data directory
			addonDataDir := filepath.Join(appDir, "addons", addonType, "data")
			if err := os.MkdirAll(addonDataDir, 0o755); err != nil {
				fmt.Printf("Warning: could not create addon data directory: %v\n", err)
			}

			_ = util.FixDirOwnership(envDir)
			_ = util.FixDirOwnership(filepath.Join(appDir, "addons", addonType)) // only the new (empty) data dir

			// Save app (generates the service in docker-compose.yml)
			if err := appManager.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// The redis/valkey images ignore *_PASSWORD_FILE: enforce the password with
			// --requirepass (kept across regenerations since "command" is not Portico-managed)
			if passwordSecret != "" {
				server := "redis-server"
				if addonType == "valkey" {
					server = "valkey-server"
				}
				command := []string{"sh", "-c", fmt.Sprintf(`exec %s --requirepass "$$(cat /run/secrets/%s)"`, server, passwordSecret)}
				dm := docker.NewManager(cfg.Registry.URL)
				if err := dm.SetServiceField(appDir, addonType, "command", command); err != nil {
					fmt.Printf("Error configuring %s password: %v\n", addonType, err)
					return
				}
			}

			if err := applyApp(cfg, appManager, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("Addon %s %s added to app %s (host: %s)\n", addonType, version, appName, host)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Version (if not specified, uses default)")
	return cmd
}
