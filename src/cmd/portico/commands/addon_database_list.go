package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonDatabaseListCmd lists databases in an addon instance
func NewAddonDatabaseListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List databases",
		Long:  "List all databases in the specified addon instance.\n\nExample:\n  portico addon database my-postgres list",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get addon-instance from parent command
			addonInstanceName, err := getAddonInstanceFromArgs(cmd)
			if err != nil || addonInstanceName == "" {
				fmt.Println("Error: addon-instance is required")
				fmt.Println("Usage: portico addon database [addon-instance] list")
				return
			}

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

			// Execute database listing command based on type
			var execCmd *exec.Cmd
			serviceName := instance.Type

			switch instance.Type {
			case "postgresql":
				// \l for listing databases
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "psql", "-U", "postgres", "-c", "\\l")
			case "mysql", "mariadb":
				// SHOW DATABASES;
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "mysql", "-u", "root", "-e", "SHOW DATABASES;")
			case "mongodb":
				// show dbs
				execCmd = exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "mongosh", "--eval", "show dbs")
			default:
				fmt.Printf("Error: unsupported database type %s\n", instance.Type)
				return
			}

			execCmd.Dir = instanceDir
			output, err := execCmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Error listing databases: %v\n%s\n", err, string(output))
				return
			}

			fmt.Printf("Databases in %s:\n", addonInstanceName)
			fmt.Println(string(output))
		},
	}

	return cmd
}
