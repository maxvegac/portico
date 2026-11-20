package commands

import (
	"fmt"

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
		DisableFlagParsing: true, // flags belong to the subcommands (e.g. image --port)
		Run:                newNestedRun("service"),
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
	appName, serviceName := nestedAppAndService(cmd)

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
