package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewServiceCmd creates the service command
func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service [app-name] [service-name] [command]",
		Short: "Manage application services",
		Long: `Manage individual services within an application.

Usage:
  portico service [app-name] [service-name] [command]

Examples:
  portico service my-app web image myregistry.com/my-app:v1.0.0
  portico service my-app web scale 3
  portico service mail-worker worker image ghcr.io/user/worker:latest --no-http-port

Note: If the app has only one service, service-name can be omitted.`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Run: func(parentCmd *cobra.Command, args []string) {
			// Parse os.Args directly since DisableFlagParsing is true
			allArgs := os.Args[1:] // Skip program name
			knownCommands := map[string]bool{
				"image": true,
				"scale": true,
			}

			var subcommandName string
			var subcommandIndex int

			// Find "service" in arguments
			serviceIndex := -1
			for i, arg := range allArgs {
				if arg == "service" {
					serviceIndex = i
					break
				}
			}

			if serviceIndex == -1 {
				_ = parentCmd.Help()
				return
			}

			// Find subcommand after "service"
			for i := serviceIndex + 1; i < len(allArgs); i++ {
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
				if subCmd.Name() == subcommandName {
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

					// Call the subcommand's Run function directly to avoid recursion
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

	// Add subcommands
	cmd.AddCommand(NewServiceUpdateImageCmd())
	cmd.AddCommand(NewServiceScaleCmd())

	return cmd
}

// getAppAndServiceFromArgs extracts app-name and service-name from service command arguments
// App-name MUST be explicit in command line arguments
// Auto-detects service-name if app has only one service
func getAppAndServiceFromArgs(cmd *cobra.Command) (string, string, error) {
	args := os.Args[1:] // Skip program name
	knownCommands := map[string]bool{
		"image": true,
		"scale": true,
	}

	var appName string
	var serviceName string

	// Extract from command line arguments - app-name MUST be explicit
	for i, arg := range args {
		if arg == "service" {
			// Next non-flag argument should be app-name
			for j := i + 1; j < len(args); j++ {
				// Skip if it's a flag
				if len(args[j]) > 0 && args[j][0] == '-' {
					continue
				}
				// Skip known commands
				if knownCommands[args[j]] {
					continue
				}
				// First non-flag, non-command should be app-name
				if appName == "" {
					appName = args[j]
				} else if serviceName == "" {
					// Second should be service-name
					serviceName = args[j]
					break
				}
			}
			break
		}
	}

	// If service-name not found and we have app-name, try to auto-detect if only one service exists
	if appName != "" && serviceName == "" {
		cfg, err := config.LoadConfig()
		if err == nil {
			appManager := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			appConfig, err := appManager.LoadApp(appName)
			if err == nil {
				// If only one service, use it
				if len(appConfig.Services) == 1 {
					serviceName = appConfig.Services[0].Name
				}
			}
		}
	}

	if appName == "" || serviceName == "" {
		return "", "", fmt.Errorf("app-name and service-name not found")
	}

	return appName, serviceName, nil
}
