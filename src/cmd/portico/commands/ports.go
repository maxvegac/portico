package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// NewPortsCmd is the root command for port mappings: ports [app-name] ...
func NewPortsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports [app-name]",
		Short: "Manage port mappings",
		Long:  "Manage port mappings for an application's services.",
		Args:  cobra.MinimumNArgs(0),
	}
	return cmd
}

// getAppNameFromPortsArgs extracts app-name from ports command arguments
// It parses os.Args to find the app-name after "ports"
func getAppNameFromPortsArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find app-name after "ports"
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "ports" {
			// Next non-flag argument should be app-name
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if args[j][0] == '-' {
					continue
				}
				// Skip known subcommands
				if args[j] == "add" || args[j] == "delete" || args[j] == "list" {
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
