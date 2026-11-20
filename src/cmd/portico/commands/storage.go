package commands

import (
	"github.com/spf13/cobra"
)

// NewStorageCmd is the root command for `storage [app-name] [service-name] <subcommand> ...`
func NewStorageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "storage [app-name] [service-name]",
		Short: "Manage storage volumes",
		Long:  "Manage storage volumes and mounts for application services.",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("storage"),
	}
}

// getAppNameFromStorageArgs extracts app-name from storage command arguments
func getAppNameFromStorageArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}

// getServiceNameFromStorageArgs extracts the optional service-name from storage command arguments
func getServiceNameFromStorageArgs(cmd *cobra.Command) (string, error) {
	_, serviceName := nestedAppAndService(cmd)
	return serviceName, nil
}
