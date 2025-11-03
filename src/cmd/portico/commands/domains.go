package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// NewDomainsCmd is the root command for domain management: domains [app-name] ...
func NewDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains [app-name]",
		Short: "Manage application domains",
		Long:  "Manage domains for an application.",
		Args:  cobra.MinimumNArgs(0),
	}
	return cmd
}

// getAppNameFromDomainsArgs extracts app-name from domains command arguments
// It parses os.Args to find the app-name after "domains"
func getAppNameFromDomainsArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find app-name after "domains"
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "domains" {
			// Next non-flag argument should be app-name
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if args[j][0] == '-' {
					continue
				}
				// Skip known subcommands
				if args[j] == "add" || args[j] == "remove" {
					continue
				}
				// This should be the app-name
				return args[j], nil
			}
			break
		}
	}
	return "", nil
}
