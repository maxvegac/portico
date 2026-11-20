package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonDatabaseListCmd lists databases in an addon instance
func NewAddonDatabaseListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List databases",
		Long:  "List all databases in the specified addon instance.\n\nExample:\n  portico addons my-postgres database list",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Println("Error: addon-instance is required")
				fmt.Println("Usage: portico addons [instance-name] database list")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			instance, instanceDir, err := loadDatabaseInstance(cfg, instanceName)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			output, err := databaseQuery(instanceDir, instance.Type, listDatabasesQuery(instance.Type))
			if err != nil {
				fmt.Printf("Error listing databases: %v\n%s\n", err, strings.TrimSpace(string(output)))
				return
			}

			fmt.Printf("Databases in %s:\n", instanceName)
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					fmt.Printf("  %s\n", line)
				}
			}
		},
	}
}
