package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonCreateCmd creates a new addon instance
func NewAddonCreateCmd() *cobra.Command {
	var addonType string
	var version string
	var mode string
	var appName string
	var instanceName string

	cmd := &cobra.Command{
		Use:   "create [instance-name]",
		Short: "Create a new addon instance",
		Long:  "Create a new addon instance (database, cache, etc.).\n\nExamples:\n  portico addon create my-postgres --type postgresql --version 16 --shared\n  portico addon create my-app-db --type postgresql --version 15 --dedicated --app my-app",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			instanceName = args[0]

			if addonType == "" {
				fmt.Println("Error: --type is required")
				fmt.Println("Available types: postgresql, mariadb, mysql, mongodb, redis, valkey")
				return
			}

			if mode == "" {
				mode = "shared" // Default to shared
			}

			if mode == "dedicated" && appName == "" {
				fmt.Println("Error: --app is required for dedicated mode")
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

			// Get version config
			versionConfig, err := def.GetVersionConfig(version)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				fmt.Printf("Available versions: %v\n", def.GetAvailableVersions())
				return
			}

			// Load existing config
			config, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			// Check if instance already exists
			if _, exists := config.Instances[instanceName]; exists {
				fmt.Printf("Error: addon instance %s already exists\n", instanceName)
				return
			}

			// Determine port (find available port)
			port := def.DefaultPort
			portInUse := make(map[int]bool)
			for _, inst := range config.Instances {
				if inst.Type == addonType {
					portInUse[inst.Port] = true
				}
			}

			// Find next available port
			for portInUse[port] {
				port++
			}

			// Create instance directory
			instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)
			dataDir := filepath.Join(instanceDir, "data")
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				fmt.Printf("Error creating instance directory: %v\n", err)
				return
			}

			// Create secrets directory
			secretsDir := filepath.Join(instanceDir, "secrets")
			if err := os.MkdirAll(secretsDir, 0o755); err != nil {
				fmt.Printf("Error creating secrets directory: %v\n", err)
				return
			}

			// Generate secrets
			for _, secretName := range versionConfig.Secrets {
				secretPath := filepath.Join(secretsDir, secretName)
				defaultValue := generateSecret(secretName)
				if err := os.WriteFile(secretPath, []byte(defaultValue), 0o600); err != nil {
					fmt.Printf("Error creating secret %s: %v\n", secretName, err)
					return
				}
			}

			// Create instance
			instance := addon.Instance{
				Name:    instanceName,
				Type:    addonType,
				Version: version,
				Mode:    mode,
				Port:    port,
				DataDir: dataDir,
			}

			if mode == "dedicated" {
				instance.App = appName
			} else {
				instance.Apps = []string{}
			}

			config.Instances[instanceName] = instance

			// Save config
			if err := am.SaveConfig(config); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				return
			}

			// Generate docker-compose.yml for the instance
			if err := generateAddonCompose(instanceDir, instance, def, versionConfig); err != nil {
				fmt.Printf("Error generating docker-compose.yml: %v\n", err)
				return
			}

			fmt.Printf("Addon instance %s created successfully!\n", instanceName)
			fmt.Printf("Type: %s, Version: %s, Mode: %s, Port: %d\n", addonType, version, mode, port)
			if mode == "dedicated" {
				fmt.Printf("Dedicated to app: %s\n", appName)
			}
		},
	}

	cmd.Flags().StringVar(&addonType, "type", "", "Addon type (postgresql, mariadb, mysql, mongodb, redis, valkey)")
	cmd.Flags().StringVar(&version, "version", "", "Version (if not specified, uses default)")
	cmd.Flags().StringVar(&mode, "mode", "shared", "Mode: shared or dedicated")
	cmd.Flags().StringVar(&appName, "app", "", "App name (required for dedicated mode)")
	return cmd
}

// generateAddonCompose generates docker-compose.yml for an addon instance
func generateAddonCompose(instanceDir string, inst addon.Instance, def *addon.Definition, versionConfig *addon.VersionConfig) error {
	composeFile := filepath.Join(instanceDir, "docker-compose.yml")

	// Build service configuration
	serviceName := inst.Type
	serviceMap := make(map[string]interface{})
	serviceMap["image"] = versionConfig.Image
	serviceMap["networks"] = []string{"portico-network"}

	// Environment variables
	env := []string{}
	for k, v := range versionConfig.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	serviceMap["environment"] = env

	// Volumes
	volumes := []string{}
	for _, vol := range versionConfig.Volumes {
		hostPath := strings.Replace(vol.HostPath, "./data", filepath.Join(instanceDir, "data"), 1)
		volumes = append(volumes, fmt.Sprintf("%s:%s", hostPath, vol.ContainerPath))
	}
	volumes = append(volumes, fmt.Sprintf("%s/secrets:/run/secrets:ro", instanceDir))
	serviceMap["volumes"] = volumes

	// Secrets
	serviceMap["secrets"] = versionConfig.Secrets

	// Ports
	ports := []string{}
	for _, portConfig := range versionConfig.Ports {
		externalPort := portConfig.External
		if externalPort == 0 {
			externalPort = inst.Port
		}
		ports = append(ports, fmt.Sprintf("%d:%d", externalPort, portConfig.Internal))
	}
	serviceMap["ports"] = ports

	// Build compose structure
	compose := map[string]interface{}{
		"services": map[string]interface{}{
			serviceName: serviceMap,
		},
		"networks": map[string]interface{}{
			"portico-network": map[string]interface{}{
				"external": true,
			},
		},
		"secrets": make(map[string]interface{}),
	}

	// Add secrets
	secretsMap := make(map[string]interface{})
	for _, secret := range versionConfig.Secrets {
		secretsMap[secret] = map[string]string{
			"file": fmt.Sprintf("./secrets/%s", secret),
		}
	}
	compose["secrets"] = secretsMap

	// Marshal to YAML
	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("error marshaling compose: %w", err)
	}

	return os.WriteFile(composeFile, data, 0o644)
}
