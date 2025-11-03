package commands

import (
	"github.com/spf13/cobra"
)

// NewAddonDatabaseCmd is the root command for database management: addon database [addon-instance] ...
func NewAddonDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database [addon-instance]",
		Short: "Manage databases in addon instances",
		Long:  "Create, delete, and list databases within addon instances (PostgreSQL, MySQL, MariaDB, MongoDB).",
		Args:  cobra.ExactArgs(1),
	}
	return cmd
}
