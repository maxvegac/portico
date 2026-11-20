package commands

import (
	"regexp"

	"github.com/spf13/cobra"
)

// envKeyRe matches valid environment variable names (no '=', spaces or newlines)
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// NewEnvCmd is the root command for `env [app-name] [service-name] <subcommand> ...`
func NewEnvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env [app-name] [service-name]",
		Short: "Manage environment variables",
		Long:  "Manage environment variables for application services.",
		Args:  cobra.ArbitraryArgs,
		// Arguments are dispatched to the subcommand (values may start with '-')
		DisableFlagParsing: true,
		Run:                newNestedRun("env"),
	}
}

// getAppNameFromEnvArgs extracts app-name from env command arguments
func getAppNameFromEnvArgs(cmd *cobra.Command) (string, error) {
	appName, _ := nestedAppAndService(cmd)
	return appName, nil
}

// getServiceNameFromEnvArgs extracts the optional service-name from env command arguments
func getServiceNameFromEnvArgs(cmd *cobra.Command) (string, error) {
	_, serviceName := nestedAppAndService(cmd)
	return serviceName, nil
}
