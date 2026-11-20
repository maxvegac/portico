package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAddonDatabaseCreateCmd creates a database in an addon instance
func NewAddonDatabaseCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [db-name]",
		Short: "Create a database",
		Long:  "Create a new database in the specified addon instance.\n\nExample:\n  portico addons my-postgres database create mydb",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Println("Error: addon-instance is required")
				fmt.Println("Usage: portico addons [instance-name] database create [db-name]")
				return
			}

			dbName := args[0]
			if err := util.ValidateDBIdentifier(dbName); err != nil {
				fmt.Printf("Error: %v\n", err)
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

			output, err := databaseQuery(instanceDir, instance.Type, createDatabaseQuery(instance.Type, dbName, databaseAppUser(instanceDir)))
			if err != nil {
				fmt.Printf("Error creating database %s: %v\n%s\n", dbName, err, strings.TrimSpace(string(output)))
				return
			}

			fmt.Printf("Database %s created successfully in %s\n", dbName, instanceName)
		},
	}
}
