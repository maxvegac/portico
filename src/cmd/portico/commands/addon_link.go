package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAddonLinkCmd links an app to an addon instance and adds environment variables
func NewAddonLinkCmd() *cobra.Command {
	var dbName string

	cmd := &cobra.Command{
		Use:   "link [addon-instance]",
		Short: "Link app to addon instance",
		Long:  "Link an application to an addon instance (database) and add connection environment variables to all services.\n\nExample:\n  portico addons my-app link my-postgres --database mydb",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get app-name from parent command (addons)
			appName, err := getAppNameFromAddonsArgs(cmd)
			if err != nil || appName == "" {
				fmt.Println("Error: app-name is required")
				fmt.Println("Usage: portico addons [app-name] link [addon-instance]")
				return
			}
			addonInstanceName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			// Load addon config
			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			addonConfig, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			instance, exists := addonConfig.Instances[addonInstanceName]
			if !exists {
				fmt.Printf("Error: addon instance %s not found\n", addonInstanceName)
				return
			}

			// Check if addon is a database type
			if !databaseTypes[instance.Type] {
				fmt.Printf("Error: addon instance %s is not a database type\n", addonInstanceName)
				return
			}

			// Load app
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := appManager.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Use default database name if not provided (app names may contain '-')
			if dbName == "" {
				dbName = strings.ReplaceAll(appName, "-", "_")
			}
			if err := util.ValidateDBIdentifier(dbName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Read secrets from addon instance
			instanceDir := filepath.Join(cfg.AddonsDir, "instances", addonInstanceName)
			secretsDir := filepath.Join(instanceDir, "secrets")

			// Read connection credentials
			dbUser := readSecret(filepath.Join(secretsDir, "db_user"))
			dbPassword := readSecret(filepath.Join(secretsDir, "db_password"))
			if dbUser == "" {
				dbUser = readSecret(filepath.Join(secretsDir, "db_name")) // Fallback
			}

			// Apps connect over portico-network: host is the instance alias, port the container port
			host := addonHost(instanceDir, instance)
			port := internalPort(am, instance)

			// Make sure the database exists
			if output, err := databaseQuery(instanceDir, instance.Type, createDatabaseQuery(instance.Type, dbName, databaseAppUser(instanceDir))); err != nil {
				if !strings.Contains(strings.ToLower(string(output)), "exists") {
					fmt.Printf("Warning: could not create database %s (is the instance running?): %s\n", dbName, strings.TrimSpace(string(output)))
				}
			}

			// Generate environment variables based on database type
			envPrefix := getEnvPrefix(instance.Type)
			envVars := map[string]string{
				envPrefix + "HOST":     host,
				envPrefix + "PORT":     strconv.Itoa(port),
				envPrefix + "DATABASE": dbName,
				envPrefix + "DB":       dbName, // Alternative name
				envPrefix + "PASSWORD": dbPassword,
			}
			userInfo := url.UserPassword(dbUser, dbPassword).String()
			switch instance.Type {
			case "postgresql":
				envVars[envPrefix+"USER"] = dbUser
				envVars[envPrefix+"URL"] = fmt.Sprintf("postgres://%s@%s:%d/%s", userInfo, host, port, dbName)
			case "mysql", "mariadb":
				envVars[envPrefix+"USER"] = dbUser
				envVars[envPrefix+"URL"] = fmt.Sprintf("mysql://%s@%s:%d/%s", userInfo, host, port, dbName)
			case "mongodb":
				envVars[envPrefix+"USERNAME"] = dbUser
				envVars[envPrefix+"URL"] = fmt.Sprintf("mongodb://%s@%s:%d/%s?authSource=admin", userInfo, host, port, dbName)
			}

			// Add environment variables to all services in the app (except inline
			// addons such as redis, which don't need database credentials)
			for i := range a.Services {
				if inlineAddonTypes[a.Services[i].Name] {
					continue
				}
				if a.Services[i].Environment == nil {
					a.Services[i].Environment = make(map[string]string)
				}
				for k, v := range envVars {
					a.Services[i].Environment[k] = v
				}
			}

			// Update addon config to link app
			if instance.Mode == "shared" {
				// Add app to shared instance
				found := false
				for _, app := range instance.Apps {
					if app == appName {
						found = true
						break
					}
				}
				if !found {
					instance.Apps = append(instance.Apps, appName)
					addonConfig.Instances[addonInstanceName] = instance
					if err := am.SaveConfig(addonConfig); err != nil {
						fmt.Printf("Warning: could not save addon config: %v\n", err)
					}
				}
			}

			// Regenerate docker-compose and redeploy
			if err := applyApp(cfg, appManager, a); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Printf("App %s linked to addon %s with database %s\n", appName, addonInstanceName, dbName)
			fmt.Printf("Environment variables added to the app services (%sURL, %sHOST, ...)\n", envPrefix, envPrefix)
		},
	}

	cmd.Flags().StringVar(&dbName, "database", "", "Database name (default: app name)")
	return cmd
}

// getEnvPrefix returns the environment variable prefix for a database type
func getEnvPrefix(dbType string) string {
	switch dbType {
	case "postgresql":
		return "POSTGRES_"
	case "mysql", "mariadb":
		return "MYSQL_"
	case "mongodb":
		return "MONGO_"
	default:
		return "DB_"
	}
}

// readSecret reads a secret file
func readSecret(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// internalPort returns the container port of an addon instance
func internalPort(am *addon.Manager, instance addon.Instance) int {
	if def, err := am.LoadDefinition(instance.Type); err == nil {
		if versionConfig, err := def.GetVersionConfig(instance.Version); err == nil && len(versionConfig.Ports) > 0 {
			return versionConfig.Ports[0].Internal
		}
		if def.DefaultPort > 0 {
			return def.DefaultPort
		}
	}
	return instance.Port
}

// addonHost returns the hostname apps use to reach an addon instance on portico-network
func addonHost(instanceDir string, instance addon.Instance) string {
	data, err := os.ReadFile(filepath.Join(instanceDir, "docker-compose.yml"))
	if err == nil {
		var compose struct {
			Name string `yaml:"name"`
		}
		if yaml.Unmarshal(data, &compose) == nil && compose.Name == util.AddonPrefix+instance.Name {
			return util.AddonPrefix + instance.Name // namespaced alias
		}
	}
	// Instances created by older versions have no alias: use the container name
	// (compose project = directory name)
	return fmt.Sprintf("%s-%s-1", instance.Name, instance.Type)
}
