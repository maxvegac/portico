package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/config"
)

// NewAppsCdCmd creates the apps cd command
func NewAppsCdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cd [app-name]",
		Short: "Change to application directory",
		Long:  "Change to the application's directory. Opens an interactive shell in the app directory.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)

			// Check if app directory exists
			if _, err := os.Stat(appDir); os.IsNotExist(err) {
				fmt.Printf("Application %s does not exist\n", appName)
				return
			}

			// Get the shell from environment, default to /bin/sh
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}

			// Print the directory for reference
			fmt.Printf("Changing to directory: %s\n", appDir)
			fmt.Printf("Type 'exit' to return to your original directory\n\n")

			// Execute interactive shell in the app directory
			cmd := exec.Command(shell)
			cmd.Dir = appDir
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				// Exit code from shell is not an error for us
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
			}
		},
	}
}
