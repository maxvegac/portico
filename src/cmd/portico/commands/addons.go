package commands

import (
	"github.com/spf13/cobra"
)

// NewAddonsCmd is the root command for addons management: addons ...
func NewAddonsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addons",
		Short: "Manage addons (databases, cache, tools)",
		Long:  "Manage addons such as databases, cache stores, and administration tools.",
		Args:  cobra.MinimumNArgs(0),
	}

	// List addons and instances
	cmd.AddCommand(NewAddonsListCmd())
	cmd.AddCommand(NewAddonsInstancesCmd())

	// Instance management (addons [instance-name] up/down/delete)
	cmd.AddCommand(NewAddonsInstanceCmd())

	// Database management subcommand
	databaseCmd := NewAddonDatabaseCmd()
	databaseCmd.AddCommand(NewAddonDatabaseCreateCmd())
	databaseCmd.AddCommand(NewAddonDatabaseDeleteCmd())
	databaseCmd.AddCommand(NewAddonDatabaseListCmd())
	cmd.AddCommand(databaseCmd)

	// Add inline addon to app
	cmd.AddCommand(NewAddonAddCmd())

	// Link/unlink app to addon
	cmd.AddCommand(NewAddonLinkCmd())

	return cmd
}
