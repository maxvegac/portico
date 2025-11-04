package commands

import (
	"github.com/spf13/cobra"
)

// NewServiceCmd creates the service command
func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service [app-name] [service-name]",
		Short: "Manage application services",
		Long:  "Manage individual services within an application.",
		Args:  cobra.MinimumNArgs(0),
	}

	// Add subcommands
	cmd.AddCommand(NewServiceUpdateImageCmd())
	cmd.AddCommand(NewServiceScaleCmd())

	return cmd
}
