package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// getAppNameFromAddonsArgs extracts app-name from addons command arguments
// It parses os.Args to find the app-name after "addons", similar to getAppNameFromDomainsArgs
func getAppNameFromAddonsArgs(_ *cobra.Command) (string, error) {
	args := os.Args[1:] // Skip program name
	knownCommands := map[string]bool{
		"list":      true,
		"instances": true,
		"create":    true,
		"database":  true,
		"add":       true,
		"link":      true,
		"up":        true,
		"down":      true,
		"delete":    true,
	}

	for i, arg := range args {
		if arg == "addons" {
			// Next non-flag argument should be app-name or instance-name or command
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if len(args[j]) > 0 && args[j][0] == '-' {
					continue
				}
				// Skip known commands
				if knownCommands[args[j]] {
					continue
				}
				// This should be the app-name or instance-name
				return args[j], nil
			}
			break
		}
	}
	return "", nil
}

// getInstanceNameFromAddonsArgs extracts instance name from addons command arguments
// It parses os.Args to find the instance name after "addons", similar to getAppNameFromDomainsArgs
func getInstanceNameFromAddonsArgs(_ *cobra.Command) (string, error) {
	args := os.Args[1:] // Skip program name
	knownCommands := map[string]bool{
		"list":      true,
		"instances": true,
		"create":    true,
		"database":  true,
		"add":       true,
		"link":      true,
		"up":        true,
		"down":      true,
		"delete":    true,
	}

	for i, arg := range args {
		if arg == "addons" {
			// Next non-flag argument should be instance name or command
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if len(args[j]) > 0 && args[j][0] == '-' {
					continue
				}
				// Skip known commands
				if knownCommands[args[j]] {
					continue
				}
				// This should be the instance name
				return args[j], nil
			}
			break
		}
	}
	return "", fmt.Errorf("instance name not found")
}
