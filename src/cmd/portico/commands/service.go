package commands

import "github.com/spf13/cobra"

// NewServiceCmd is the root command for service port mappings: service <app> ...
func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service [app-name]",
		Short: "Manage service port mappings",
		Long:  "Manage service port mappings for an application's services.",
		Args:  cobra.MinimumNArgs(1),
	}
	return cmd
}
