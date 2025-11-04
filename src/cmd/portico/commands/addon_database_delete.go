package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonDatabaseDeleteCmd deletes a database from an addon instance
func NewAddonDatabaseDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [db-name]",
		Short: "Delete a database",
		Long:  "Delete a database from the specified addon instance.\n\nExample:\n  portico addon database my-postgres delete mydb",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get addon-instance from parent command (addons)
			addonInstanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || addonInstanceName == "" {
				fmt.Println("Error: addon-instance is required")
				fmt.Println("Usage: portico addons [instance-name] database delete [db-name]")
				return
			}

			dbName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			config, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			instance, exists := config.Instances[addonInstanceName]
			if !exists {
				fmt.Printf("Error: addon instance %s not found\n", addonInstanceName)
				return
			}

			// Check if addon is a database type
			if instance.Type != "postgresql" && instance.Type != "mysql" && instance.Type != "mariadb" && instance.Type != "mongodb" {
				fmt.Printf("Error: addon instance %s is not a database type\n", addonInstanceName)
				return
			}

			instanceDir := filepath.Join(cfg.AddonsDir, "instances", addonInstanceName)
			composeFile := filepath.Join(instanceDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("Error: docker-compose.yml not found for instance %s\n", addonInstanceName)
				return
			}

			// Execute database deletion command based on type
			var execCmd *exec.Cmd
			serviceName := instance.Type

			switch instance.Type {
			case "postgresql":
				// DROP DATABASE dbname;
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "psql", "-U", "postgres", "-c", fmt.Sprintf("DROP DATABASE %s;", dbName))
			case "mysql", "mariadb":
				// DROP DATABASE dbname;
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "mysql", "-u", "root", "-e", fmt.Sprintf("DROP DATABASE %s;", dbName))
			case "mongodb":
				// use dbname; db.dropDatabase();
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "mongosh", "--eval", fmt.Sprintf("use %s; db.dropDatabase();", dbName))
			default:
				fmt.Printf("Error: unsupported database type %s\n", instance.Type)
				return
			}

			execCmd.Dir = instanceDir
			output, err := execCmd.CombinedOutput()
			if err != nil {
				if strings.Contains(string(output), "does not exist") || strings.Contains(string(output), "ERROR") {
					fmt.Printf("Database %s may not exist or error occurred: %s\n", dbName, string(output))
					return
				}
				fmt.Printf("Error deleting database: %v\n%s\n", err, string(output))
				return
			}

			fmt.Printf("Database %s deleted successfully from %s\n", dbName, addonInstanceName)
		},
	}

	return cmd
}
