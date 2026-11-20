package commands

import (
	"github.com/spf13/cobra"
)

// NewSecretsCmd is the root command for `secrets [app-name] [service-name] <subcommand> ...`
func NewSecretsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secrets [app-name] [service-name]",
		Short: "Manage secrets",
		Long:  "Manage secrets (files in env/ directory) for application services.",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("secrets"),
	}
}

// getAppNameFromSecretsArgs extracts app-name from secrets command arguments
func getAppNameFromSecretsArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}

// getServiceNameFromSecretsArgs extracts the optional service-name from secrets command arguments
func getServiceNameFromSecretsArgs(cmd *cobra.Command) (string, error) {
	_, serviceName := nestedAppAndService(cmd)
	return serviceName, nil
}
