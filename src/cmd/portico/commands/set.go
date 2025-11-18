package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewSetCmd is the root command for app configuration: set [app-name] ...
func NewSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [app-name] [property] [value]",
		Short: "Set application configuration",
		Long:  "Set application configuration properties.",
		Args:  cobra.ArbitraryArgs,
		Run: func(parentCmd *cobra.Command, args []string) {
			// Parse os.Args directly to find subcommand
			allArgs := os.Args[1:] // Skip program name
			knownProperties := map[string]bool{
				"http-port":    true,
				"http-service": true,
				"http":         true,
			}

			var propertyName string
			var propertyIndex int

			// Find "set" in arguments
			setIndex := -1
			for i, arg := range allArgs {
				if arg == "set" {
					setIndex = i
					break
				}
			}

			if setIndex == -1 {
				_ = parentCmd.Help()
				return
			}

			// Find property after "set" and app-name
			for i := setIndex + 1; i < len(allArgs); i++ {
				if knownProperties[allArgs[i]] {
					propertyName = allArgs[i]
					propertyIndex = i
					break
				}
			}

			// If no property found, show help
			if propertyName == "" {
				_ = parentCmd.Help()
				return
			}

			// Find and execute subcommand
			for _, subCmd := range parentCmd.Commands() {
				if subCmd.Name() == propertyName {
					// Get arguments for subcommand (everything after property name)
					subcommandArgs := allArgs[propertyIndex+1:]

					// Parse flags manually for the subcommand
					if err := subCmd.ParseFlags(subcommandArgs); err != nil {
						fmt.Printf("Error parsing flags: %v\n", err)
						_ = subCmd.Help()
						return
					}

					// Get non-flag arguments
					nonFlagArgs := subCmd.Flags().Args()

					// Call the subcommand's Run function directly
					if subCmd.Run != nil {
						subCmd.Run(subCmd, nonFlagArgs)
					} else if subCmd.RunE != nil {
						if err := subCmd.RunE(subCmd, nonFlagArgs); err != nil {
							fmt.Printf("Error: %v\n", err)
							_ = subCmd.Help()
						}
					} else {
						_ = subCmd.Help()
					}
					return
				}
			}

			// Property not found
			_ = parentCmd.Help()
		},
	}
	return cmd
}

// getAppNameFromSetArgs extracts app-name from set command arguments
// It parses os.Args to find the app-name after "set"
func getAppNameFromSetArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find app-name after "set"
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "set" {
			// Next non-flag argument should be app-name
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if args[j][0] == '-' {
					continue
				}
				// Skip known properties
				if args[j] == "http-port" || args[j] == "http-service" || args[j] == "http" {
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
