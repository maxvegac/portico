package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// NewStorageCmd is the root command for volume/storage management: storage [app-name] ...
func NewStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage [app-name]",
		Short: "Manage storage volumes",
		Long:  "Manage storage volumes and mounts for application services.",
		Args:  cobra.MinimumNArgs(0),
	}
	return cmd
}

// getAppNameFromStorageArgs extracts app-name from storage command arguments
// It parses os.Args to find the app-name after "storage"
func getAppNameFromStorageArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find app-name after "storage"
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "storage" {
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
