package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAppsExecCmd creates the apps exec command
func NewAppsExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [app-name] [[service] [command...]]",
		Short: "Execute command in application container",
		Long:  "Execute a command in a service container. If service name is provided as second argument, it will be used. Otherwise, uses default service.\n\nExamples:\n  portico exec my-app ls -la\n  portico exec my-app database psql -U postgres",
		Args:  cobra.MinimumNArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]
			remainingArgs := args[1:]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := app.NewManager(cfg.AppsDir, cfg.TemplatesDir)
			a, err := am.LoadApp(appName)
			if err != nil {
				fmt.Printf("Error loading app: %v\n", err)
				return
			}

			// Get list of service names
			serviceNames := make(map[string]bool)
			for _, s := range a.Services {
				serviceNames[s.Name] = true
			}

			var serviceName string
			var command []string

			// Determine service and command from arguments
			if len(remainingArgs) == 0 {
				fmt.Println("Error: command is required")
				fmt.Println("Usage: portico exec [app-name] [[service] [command...]]")
				return
			}

			// Check if first argument is a service name
			if len(remainingArgs) > 1 && serviceNames[remainingArgs[0]] {
				// Second argument is service name
				serviceName = remainingArgs[0]
				command = remainingArgs[1:]
			} else {
				// No service specified, use default
				if len(a.Services) == 1 {
					serviceName = a.Services[0].Name
					command = remainingArgs
				} else {
					fmt.Printf("Error: app %s has %d services. Please specify service name\n", appName, len(a.Services))
					var names []string
					for _, s := range a.Services {
						names = append(names, s.Name)
					}
					fmt.Printf("Available services: %v\n", names)
					fmt.Printf("Usage: portico exec %s [service] [command...]\n", appName)
					return
				}
			}

			if len(command) == 0 {
				fmt.Println("Error: command is required")
				fmt.Println("Usage: portico exec [app-name] [[service] [command...]]")
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			composeFile := filepath.Join(appDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("docker-compose.yml not found for app %s\n", appName)
				return
			}

			// Build docker compose exec command
			execArgs := []string{"compose", "-f", composeFile, "exec", serviceName}
			execArgs = append(execArgs, command...)

			cmd := exec.Command("docker", execArgs...)
			cmd.Dir = appDir
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				fmt.Printf("Error executing command: %v\n", err)
				return
			}
		},
	}

	return cmd
}
