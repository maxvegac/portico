package commands

import "github.com/spf13/cobra"

// NewAppsSetCmd is the root command for configuration setters: apps set ...
func NewAppsSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Configure application settings",
		Long:  "Configure application settings such as ports and domains.",
	}

	// set port ...
	port := NewAppsSetPortRootCmd()
	port.AddCommand(NewAppsSetHTTPPortCmd())
	port.AddCommand(NewAppsSetServicePortCmd())
	cmd.AddCommand(port)

	return cmd
}
