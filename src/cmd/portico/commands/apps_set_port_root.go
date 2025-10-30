package commands

import "github.com/spf13/cobra"

// NewAppsSetPortRootCmd groups port-related setters: apps set port ...
func NewAppsSetPortRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port",
		Short: "Configure application ports",
		Long:  "Configure application ports such as the HTTP (proxy) port and per-service ports.",
	}
}
