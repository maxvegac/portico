package commands

import (
	"github.com/spf13/cobra"
)

// NewAppsCmd creates the apps command
func NewAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage applications",
		Long:  `Manage applications deployed on Portico platform.`,
	}

	// Subcommands
	cmd.AddCommand(NewAppsCreateCmd())
	cmd.AddCommand(NewAppsListCmd())
	cmd.AddCommand(NewAppsResetCmd())
	cmd.AddCommand(NewAppsDestroyCmd())
	cmd.AddCommand(NewAppsUpCmd())
	cmd.AddCommand(NewAppsDownCmd())
	cmd.AddCommand(NewAppsSetDomainCmd())
	// Top-level ports command (not used anymore, but kept for backwards compatibility)
	ports := NewPortsCmd()
	ports.AddCommand(NewPortsAddCmd())
	ports.AddCommand(NewPortsDeleteCmd())
	ports.AddCommand(NewPortsListCmd())
	cmd.AddCommand(ports)

	return cmd
}
