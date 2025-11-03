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
)

// NewAddonAddCmd adds an inline addon (redis/valkey) as a service to an app
func NewAddonAddCmd() *cobra.Command {
	var addonType string
	var version string

	cmd := &cobra.Command{
		Use:   "add [app-name] [addon-type]",
		Short: "Add inline addon (redis/valkey) as service to app",
		Long:  "Add an inline addon (redis or valkey) as a service within an application.\n\nExample:\n  portico addon add my-app redis --version 7",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			addonType = args[1]

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

			// Add ports
			for _, portConfig := range versionConfig.Ports {
				if portConfig.External > 0 {
					newService.ExtraPorts = append(newService.ExtraPorts, fmt.Sprintf("%d:%d", portConfig.External, portConfig.Internal))
				}
			}

			// Create secrets directory and files first
			appDir := filepath.Join(cfg.AppsDir, appName)
			envDir := filepath.Join(appDir, "env")

			var addonPassword string
			for _, secretName := range versionConfig.Secrets {
				secretPath := filepath.Join(envDir, secretName)
				defaultValue := generateSecret(secretName)
				if err := os.WriteFile(secretPath, []byte(defaultValue), 0o600); err != nil {
					fmt.Printf("Warning: could not create secret %s: %v\n", secretName, err)
				}
				if strings.Contains(strings.ToLower(secretName), "password") {
					addonPassword = defaultValue
				}
			}

			// Add service to app
			a.Services = append(a.Services, newService)

			// Add environment variables to other services for connection
			envPrefix := strings.ToUpper(addonType)
			for i := range a.Services {
				if a.Services[i].Name != addonType { // Don't add to the addon service itself
					if a.Services[i].Environment == nil {
						a.Services[i].Environment = make(map[string]string)
					}
					a.Services[i].Environment[envPrefix+"_HOST"] = addonType
					a.Services[i].Environment[envPrefix+"_PORT"] = fmt.Sprintf("%d", def.DefaultPort)
					if addonPassword != "" {
						a.Services[i].Environment[envPrefix+"_PASSWORD"] = addonPassword
					}
				}
			}

			// Save app
			if err := appManager.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Create addon data directory
			addonDataDir := filepath.Join(appDir, "addons", addonType, "data")
			if err := os.MkdirAll(addonDataDir, 0o755); err != nil {
				fmt.Printf("Warning: could not create addon data directory: %v\n", err)
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)
			var dockerServices []docker.Service
			for _, s := range a.Services {
				dockerServices = append(dockerServices, docker.Service{
					Name:        s.Name,
					Image:       s.Image,
					Port:        s.Port,
					ExtraPorts:  s.ExtraPorts,
					Environment: s.Environment,
					Volumes:     s.Volumes,
					Secrets:     s.Secrets,
					DependsOn:   s.DependsOn,
				})
			}

			metadata := &docker.PorticoMetadata{
				Domain: a.Domain,
				Port:   a.Port,
			}

			if err := dm.GenerateDockerCompose(appDir, dockerServices, metadata); err != nil {
				fmt.Printf("Error generating docker compose: %v\n", err)
				return
			}

			if err := dm.DeployApp(appDir); err != nil {
				fmt.Printf("Error deploying app: %v\n", err)
				return
			}

			fmt.Printf("Addon %s (version %s) added to app %s\n", addonType, version, appName)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Version (if not specified, uses default)")
	return cmd
}
