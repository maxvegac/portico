package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewAddonLinkCmd links an app to an addon instance and adds environment variables
func NewAddonLinkCmd() *cobra.Command {
	var dbName string

	cmd := &cobra.Command{
		Use:   "link [app-name] [addon-instance]",
		Short: "Link app to addon instance",
		Long:  "Link an application to an addon instance (database) and add connection environment variables to all services.\n\nExample:\n  portico addon link my-app my-postgres --database mydb",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			addonInstanceName := args[1]

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
			if instance.Type != "postgresql" && instance.Type != "mysql" && instance.Type != "mariadb" && instance.Type != "mongodb" {
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

			// Use default database name if not provided
			if dbName == "" {
				dbName = appName // Default to app name
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

			// Generate environment variables based on database type
			envPrefix := getEnvPrefix(instance.Type)
			envVars := make(map[string]string)

			switch instance.Type {
			case "postgresql":
				envVars[envPrefix+"HOST"] = addonInstanceName
				envVars[envPrefix+"PORT"] = strconv.Itoa(instance.Port)
				envVars[envPrefix+"DATABASE"] = dbName
				envVars[envPrefix+"USER"] = dbUser
				envVars[envPrefix+"PASSWORD"] = dbPassword
				envVars[envPrefix+"DB"] = dbName // Alternative name
			case "mysql", "mariadb":
				envVars[envPrefix+"HOST"] = addonInstanceName
				envVars[envPrefix+"PORT"] = strconv.Itoa(instance.Port)
				envVars[envPrefix+"DATABASE"] = dbName
				envVars[envPrefix+"DB"] = dbName
				envVars[envPrefix+"USER"] = dbUser
				envVars[envPrefix+"PASSWORD"] = dbPassword
			case "mongodb":
				envVars[envPrefix+"HOST"] = addonInstanceName
				envVars[envPrefix+"PORT"] = strconv.Itoa(instance.Port)
				envVars[envPrefix+"DATABASE"] = dbName
				envVars[envPrefix+"DB"] = dbName
				envVars[envPrefix+"USERNAME"] = dbUser
				envVars[envPrefix+"PASSWORD"] = dbPassword
			}

			// Add environment variables to all services in the app
			for i := range a.Services {
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

			// Save app
			if err := appManager.SaveApp(a); err != nil {
				fmt.Printf("Error saving app: %v\n", err)
				return
			}

			// Regenerate docker-compose and redeploy
			dm := docker.NewManager(cfg.Registry.URL)
			appDir := filepath.Join(cfg.AppsDir, appName)
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

			fmt.Printf("App %s linked to addon %s with database %s\n", appName, addonInstanceName, dbName)
			fmt.Printf("Environment variables added to all services\n")
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
	return string(data)
}
