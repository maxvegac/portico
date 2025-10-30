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
	cmd.AddCommand(NewAppsDeployCmd())
	cmd.AddCommand(NewAppsDestroyCmd())
	cmd.AddCommand(NewAppsUpCmd())
	cmd.AddCommand(NewAppsDownCmd())
	cmd.AddCommand(NewAppsSetDomainCmd())
	cmd.AddCommand(NewAppsSetCmd())
	// Top-level service command
	svc := NewServiceCmd()
	svc.AddCommand(NewServiceHTTPCmd())
	svc.AddCommand(NewServiceAddCmd())
	svc.AddCommand(NewServiceDeleteCmd())
	svc.AddCommand(NewServiceListCmd())
	cmd.AddCommand(svc)

	return cmd
}
