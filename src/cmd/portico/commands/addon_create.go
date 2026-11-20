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
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAddonCreateCmd creates a new addon instance
func NewAddonCreateCmd() *cobra.Command {
	var addonType string
	var version string
	var mode string
	var appName string
	var instanceName string
	var target string
	var domain string

	cmd := &cobra.Command{
		Use:   "create [instance-name]",
		Short: "Create a new addon instance",
		Long: `Create a new addon instance (database, cache or admin tool).

Admin tools (adminer, mongo-express) listen on 127.0.0.1 only, reachable through an
SSH tunnel. With --domain they are also published by Caddy behind HTTP basic auth.

Examples:
  portico addons create my-postgres --type postgresql --version 18
  portico addons create my-app-db --type postgresql --mode dedicated --app my-app
  portico addons create dbadmin --type adminer
  portico addons create dbadmin --type adminer --target my-postgres --domain db.example.com
  portico addons create mongoadmin --type mongo-express --target my-mongo`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			instanceName = args[0]
			// Instance names become directories, compose projects and DNS aliases
			if err := util.ValidateAppName(instanceName); err != nil {
				fmt.Printf("Error: invalid instance name: %v\n", err)
				return
			}

			if addonType == "" {
				fmt.Println("Error: --type is required")
				fmt.Printf("Available types: %s\n", strings.Join(addonTypeNames, ", "))
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
			if version == "" {
				version = def.LatestVersion()
			}
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

			isTool := def.Type == "tool"
			if !isTool && (target != "" || domain != "") {
				fmt.Println("Error: --target and --domain only apply to admin tools (adminer, mongo-express)")
				return
			}
			if isTool && mode == "dedicated" {
				fmt.Println("Error: admin tools can't be dedicated to an app")
				return
			}
			if def.ServiceMode == "inline" {
				fmt.Printf("Error: %s is added to an app: portico addons <app> add %s\n", addonType, addonType)
				return
			}

			// Determine host port (find one not used by another instance)
			port := def.DefaultPort
			portInUse := make(map[int]bool)
			for _, inst := range config.Instances {
				portInUse[inst.Port] = true
			}

			// Find next available port
			for portInUse[port] {
				port++
			}

			if isTool {
				instance := addon.Instance{
					Name:    instanceName,
					Type:    addonType,
					Version: version,
					Mode:    "shared",
					Port:    port,
					Domain:  strings.ToLower(domain),
					Target:  target,
				}
				instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)
				password, err := createToolInstance(cfg, am, config.Instances, &instance, versionConfig)
				if err != nil {
					_ = os.RemoveAll(instanceDir)
					fmt.Printf("Error: %v\n", err)
					return
				}
				config.Instances[instanceName] = instance
				if err := am.SaveConfig(config); err != nil {
					_ = os.RemoveAll(instanceDir)
					fmt.Printf("Error saving config: %v\n", err)
					return
				}
				_ = util.FixDirOwnership(instanceDir)
				_ = util.FixFileOwnership(am.ConfigFile)
				if instance.Domain != "" {
					if err := publishTool(cfg, instance, versionConfig.Ports[0].Internal, password); err != nil {
						fmt.Printf("Error publishing %s: %v\n", instance.Domain, err)
						return
					}
				}
				fmt.Printf("Admin tool %s (%s %s) created successfully!\n", instanceName, addonType, version)
				printToolAccess(instance, password)
				return
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
				defaultValue, err := generateSecret(secretName)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
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

			_ = util.FixDirOwnership(instanceDir)
			_ = util.FixFileOwnership(am.ConfigFile)

			fmt.Printf("Addon instance %s created successfully!\n", instanceName)
			fmt.Printf("Type: %s, Version: %s, Mode: %s, Port: %d\n", addonType, version, mode, port)
			if mode == "dedicated" {
				fmt.Printf("Dedicated to app: %s\n", appName)
			}
		},
	}

	cmd.Flags().StringVar(&addonType, "type", "", "Addon type ("+strings.Join(addonTypeNames, ", ")+")")
	cmd.Flags().StringVar(&version, "version", "", "Version (if not specified, uses default)")
	cmd.Flags().StringVar(&mode, "mode", "shared", "Mode: shared or dedicated")
	cmd.Flags().StringVar(&appName, "app", "", "App name (required for dedicated mode)")
	cmd.Flags().StringVar(&target, "target", "", "Admin tools: database instance to manage (required for mongo-express)")
	cmd.Flags().StringVar(&domain, "domain", "", "Admin tools: publish on this domain with HTTPS and basic auth")
	return cmd
}

// generateAddonCompose generates docker-compose.yml for an addon instance
func generateAddonCompose(instanceDir string, inst addon.Instance, def *addon.Definition, versionConfig *addon.VersionConfig) error {
	composeFile := filepath.Join(instanceDir, "docker-compose.yml")

	// Build service configuration
	serviceName := inst.Type
	serviceMap := make(map[string]interface{})
	serviceMap["image"] = versionConfig.Image
	serviceMap["restart"] = "unless-stopped"
	// Apps reach the instance over portico-network by its namespaced alias
	serviceMap["networks"] = map[string]interface{}{
		"portico-network": map[string]interface{}{
			"aliases": []string{util.AddonPrefix + inst.Name},
		},
	}

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

	// Ports: published on localhost only (for admin tools / SSH tunnels).
	// The main port uses the instance's allocated host port so several instances
	// of the same type don't collide.
	ports := []string{}
	for i, portConfig := range versionConfig.Ports {
		hostPort := portConfig.External
		if i == 0 || hostPort == 0 {
			hostPort = inst.Port
		}
		ports = append(ports, fmt.Sprintf("127.0.0.1:%d:%d", hostPort, portConfig.Internal))
	}
	serviceMap["ports"] = ports

	// Build compose structure
	compose := map[string]interface{}{
		"name": util.AddonPrefix + inst.Name, // never collides with an app's compose project
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

	return util.WriteFileAtomic(composeFile, data, 0o644)
}
