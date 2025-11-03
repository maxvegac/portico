package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAppsShellCmd creates the apps shell command
func NewAppsShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell [app-name] [[service] [shell]]",
		Short: "Open interactive shell in application container",
		Long:  "Open an interactive shell in a service container. If service name is provided as second argument, it will be used. If shell is provided as third argument, it will be used. Otherwise auto-detects.\n\nExamples:\n  portico shell my-app\n  portico shell my-app database\n  portico shell my-app database bash",
		Args:  cobra.MinimumNArgs(1),
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
			var shell string

			// Determine service and shell from arguments
			if len(remainingArgs) == 0 {
				// No arguments: use default service, auto-detect shell
				if len(a.Services) == 1 {
					serviceName = a.Services[0].Name
				} else {
					fmt.Printf("Error: app %s has %d services. Please specify service name\n", appName, len(a.Services))
					var names []string
					for _, s := range a.Services {
						names = append(names, s.Name)
					}
					fmt.Printf("Available services: %v\n", names)
					fmt.Printf("Usage: portico shell %s [service] [shell]\n", appName)
					return
				}
			} else if len(remainingArgs) == 1 {
				// One argument: could be service name or shell
				if serviceNames[remainingArgs[0]] {
					// It's a service name
					serviceName = remainingArgs[0]
					// Shell will be auto-detected
				} else {
					// It's a shell name, use default service
					if len(a.Services) == 1 {
						serviceName = a.Services[0].Name
						shell = remainingArgs[0]
					} else {
						fmt.Printf("Error: app %s has %d services. Please specify service name\n", appName, len(a.Services))
						var names []string
						for _, s := range a.Services {
							names = append(names, s.Name)
						}
						fmt.Printf("Available services: %v\n", names)
						fmt.Printf("Usage: portico shell %s [service] [shell]\n", appName)
						return
					}
				}
			} else {
				// Two or more arguments: first is service, second is shell
				if serviceNames[remainingArgs[0]] {
					serviceName = remainingArgs[0]
					shell = remainingArgs[1]
				} else {
					fmt.Printf("Error: '%s' is not a valid service name\n", remainingArgs[0])
					var names []string
					for _, s := range a.Services {
						names = append(names, s.Name)
					}
					fmt.Printf("Available services: %v\n", names)
					return
				}
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			composeFile := filepath.Join(appDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("docker-compose.yml not found for app %s\n", appName)
				return
			}

			// Determine shell to use if not specified
			if shell == "" {
				// Try common shells in order of preference
				shells := []string{"bash", "sh", "/bin/bash", "/bin/sh"}
				for _, s := range shells {
					// Check if shell exists in container by trying to exec it
					testCmd := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", serviceName, "which", s)
					testCmd.Dir = appDir
					testCmd.Stdout = nil
					testCmd.Stderr = nil
					if err := testCmd.Run(); err == nil {
						shell = s
						break
					}
				}
				if shell == "" {
					shell = "sh" // Default fallback
				}
			}

			// Build docker compose exec command with -it flags
			execArgs := []string{"compose", "-f", composeFile, "exec", "-it", serviceName}

			// Split shell command if it contains spaces (e.g., "bash -l")
			shellParts := strings.Fields(shell)
			execArgs = append(execArgs, shellParts...)

			cmd := exec.Command("docker", execArgs...)
			cmd.Dir = appDir
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				fmt.Printf("Error opening shell: %v\n", err)
				return
			}
		},
	}

	return cmd
}
