package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// NewAddonDatabaseDeleteCmd deletes a database from an addon instance
func NewAddonDatabaseDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [db-name]",
		Short: "Delete a database",
		Long:  "Delete a database from the specified addon instance. Asks for confirmation unless --force is given.\n\nExample:\n  portico addons my-postgres database delete mydb",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Println("Error: addon-instance is required")
				fmt.Println("Usage: portico addons [instance-name] database delete [db-name]")
				return
			}

			dbName := args[0]
			if err := util.ValidateDBIdentifier(dbName); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			if force, _ := cmd.Flags().GetBool("force"); !force {
				if !confirm(fmt.Sprintf("This will permanently delete database %s and all its data. Continue?", dbName)) {
					fmt.Println("Cancelled.")
					return
				}
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

			output, err := databaseQuery(instanceDir, instance.Type, dropDatabaseQuery(instance.Type, dbName))
			if err != nil {
				fmt.Printf("Error deleting database %s: %v\n%s\n", dbName, err, strings.TrimSpace(string(output)))
				return
			}

			fmt.Printf("Database %s deleted successfully from %s\n", dbName, instanceName)
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Do not ask for confirmation")
	return cmd
}
