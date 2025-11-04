package commands

import (
	"github.com/spf13/cobra"
)

// NewAddonDatabaseCmd is the root command for database management: addons [instance-name] database ...
func NewAddonDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Manage databases in addon instances",
		Long:  "Create, delete, and list databases within addon instances (PostgreSQL, MySQL, MariaDB, MongoDB).\n\nExample:\n  portico addons my-postgres database create mydb",
		Args:  cobra.NoArgs,
	}
	return cmd
}
