package commands

import (
	"github.com/spf13/cobra"
)

// NewSetCmd is the root command for app configuration: set [app-name] ...
func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [app-name] [property] [value]",
		Short: "Set application configuration",
		Long:  "Set application configuration properties.",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("set"),
	}
}

// getAppNameFromSetArgs extracts app-name from set command arguments
func getAppNameFromSetArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}
