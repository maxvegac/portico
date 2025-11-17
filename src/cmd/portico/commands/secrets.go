package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewSecretsCmd is the root command for secrets: secrets [app-name] ...
func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets [app-name] [service-name]",
		Short: "Manage secrets",
		Long:  "Manage secrets (files in env/ directory) for application services.",
		Args:  cobra.ArbitraryArgs,
		Run: func(parentCmd *cobra.Command, args []string) {
			// Parse os.Args directly to find subcommand
			allArgs := os.Args[1:] // Skip program name
			knownCommands := map[string]bool{
				"add":    true,
				"del":    true,
				"delete": true,
				"edit":   true,
				"list":   true,
			}

			var subcommandName string
			var subcommandIndex int

			// Find "secrets" in arguments
			secretsIndex := -1
			for i, arg := range allArgs {
				if arg == "secrets" {
					secretsIndex = i
					break
				}
			}

			if secretsIndex == -1 {
				_ = parentCmd.Help()
				return
			}

			// Find subcommand after "secrets"
			for i := secretsIndex + 1; i < len(allArgs); i++ {
				if knownCommands[allArgs[i]] {
					subcommandName = allArgs[i]
					subcommandIndex = i
					break
				}
			}

			// If no subcommand found, show help
			if subcommandName == "" {
				_ = parentCmd.Help()
				return
			}

			// Find and execute subcommand
			for _, subCmd := range parentCmd.Commands() {
				if subCmd.Name() == subcommandName || (subcommandName == "delete" && subCmd.Name() == "del") {
					// Get arguments for subcommand (everything after subcommand name)
					subcommandArgs := allArgs[subcommandIndex+1:]

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

			// Subcommand not found
			_ = parentCmd.Help()
		},
	}
	return cmd
}

// getAppNameFromSecretsArgs extracts app-name from secrets command arguments
// It parses os.Args to find the app-name after "secrets"
func getAppNameFromSecretsArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find app-name after "secrets"
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "secrets" {
			// Next non-flag argument should be app-name
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if args[j][0] == '-' {
					continue
				}
				// Skip known subcommands
				if args[j] == "add" || args[j] == "del" || args[j] == "delete" || args[j] == "edit" || args[j] == "list" {
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

// getServiceNameFromSecretsArgs extracts service-name from secrets command arguments
// It parses os.Args to find the service-name after "secrets" and app-name
func getServiceNameFromSecretsArgs(cmd *cobra.Command) (string, error) {
	// Parse os.Args to find service-name after "secrets" and app-name
	args := os.Args[1:] // Skip program name
	for i, arg := range args {
		if arg == "secrets" {
			// Find app-name first
			appNameFound := false
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if args[j][0] == '-' {
					continue
				}
				// Skip known subcommands
				if args[j] == "add" || args[j] == "del" || args[j] == "delete" || args[j] == "edit" || args[j] == "list" {
					continue
				}
				if !appNameFound {
					appNameFound = true
					continue
				}
				// This should be the service-name
				return args[j], nil
			}
			break
		}
	}
	return "", nil
}
