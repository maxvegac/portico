package commands

import (
	"github.com/spf13/cobra"
)

// NewPortsCmd is the root command for `ports [app-name] [service-name] <subcommand> ...`
func NewPortsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ports [app-name] [service-name]",
		Short: "Manage port mappings",
		Long:  "Manage port mappings for an application's services.",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("ports"),
	}
}

// getAppNameFromPortsArgs extracts app-name from ports command arguments
func getAppNameFromPortsArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}

// getServiceNameFromPortsArgs extracts the optional service-name from ports command arguments
func getServiceNameFromPortsArgs(cmd *cobra.Command) (string, error) {
	_, serviceName := nestedAppAndService(cmd)
	return serviceName, nil
}
