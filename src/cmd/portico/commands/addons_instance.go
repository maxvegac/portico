package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
)

// NewAddonsInstanceCmd creates a command for managing a specific addon instance
func NewAddonsInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "[instance-name]",
		Short: "Manage addon instance",
		Long:  "Manage a specific addon instance (up, down, delete).\n\nExample:\n  portico addons psql18 up",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			cfg, err := config.LoadConfig()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			addonConfig, err := am.LoadConfig()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}

			var instances []string
			for name := range addonConfig.Instances {
				instances = append(instances, name)
			}
			return instances, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Show help if no subcommand is provided
			_ = cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewAddonsInstanceUpCmd())
	cmd.AddCommand(NewAddonsInstanceDownCmd())
	cmd.AddCommand(NewAddonsInstanceDeleteCmd())

	return cmd
}

// NewAddonsInstanceUpCmd starts an addon instance
func NewAddonsInstanceUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start addon instance",
		Long:  "Start an addon instance using docker compose up -d.",
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Printf("Error: instance name required\n")
				fmt.Printf("Usage: portico addons [instance-name] up\n")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			addonConfig, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			if _, exists := addonConfig.Instances[instanceName]; !exists {
				fmt.Printf("Error: addon instance %s not found\n", instanceName)
				return
			}

			instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)
			composeFile := filepath.Join(instanceDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("Error: docker-compose.yml not found for instance %s\n", instanceName)
				return
			}

			// Run docker compose up
			execCmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
			execCmd.Dir = instanceDir

			output, err := execCmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Error starting addon instance: %v\n%s\n", err, string(output))
				return
			}

			fmt.Printf("Addon instance %s started successfully\n", instanceName)
		},
	}

	return cmd
}

// NewAddonsInstanceDownCmd stops an addon instance
func NewAddonsInstanceDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop addon instance",
		Long:  "Stop an addon instance using docker compose down.",
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Printf("Error: instance name required\n")
				fmt.Printf("Usage: portico addons [instance-name] down\n")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			addonConfig, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			if _, exists := addonConfig.Instances[instanceName]; !exists {
				fmt.Printf("Error: addon instance %s not found\n", instanceName)
				return
			}

			instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)
			composeFile := filepath.Join(instanceDir, "docker-compose.yml")

			// Check if compose file exists
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				fmt.Printf("Error: docker-compose.yml not found for instance %s\n", instanceName)
				return
			}

			// Run docker compose down
			execCmd := exec.Command("docker", "compose", "-f", composeFile, "down")
			execCmd.Dir = instanceDir

			output, err := execCmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Error stopping addon instance: %v\n%s\n", err, string(output))
				return
			}

			fmt.Printf("Addon instance %s stopped successfully\n", instanceName)
		},
	}

	return cmd
}

// NewAddonsInstanceDeleteCmd deletes an addon instance
func NewAddonsInstanceDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete addon instance",
		Long:  "Delete an addon instance and its data. This will stop and remove the instance.",
		Run: func(cmd *cobra.Command, args []string) {
			instanceName, err := getInstanceNameFromAddonsArgs(cmd)
			if err != nil || instanceName == "" {
				fmt.Printf("Error: instance name required\n")
				fmt.Printf("Usage: portico addons [instance-name] delete\n")
				return
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
			addonConfig, err := am.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading addons config: %v\n", err)
				return
			}

			if _, exists := addonConfig.Instances[instanceName]; !exists {
				fmt.Printf("Error: addon instance %s not found\n", instanceName)
				return
			}

			instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)

			// Stop and remove containers first
			composeFile := filepath.Join(instanceDir, "docker-compose.yml")
			if _, err := os.Stat(composeFile); err == nil {
				fmt.Printf("Stopping instance %s...\n", instanceName)
				// Run docker compose down to stop containers
				execCmd := exec.Command("docker", "compose", "-f", composeFile, "down")
				execCmd.Dir = instanceDir
				if err := execCmd.Run(); err != nil {
					fmt.Printf("Warning: could not stop containers: %v\n", err)
				}
			}

			// Remove from config
			delete(addonConfig.Instances, instanceName)
			if err := am.SaveConfig(addonConfig); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				return
			}

			// Remove instance directory
			if err := os.RemoveAll(instanceDir); err != nil {
				fmt.Printf("Warning: could not remove instance directory: %v\n", err)
			}

			fmt.Printf("Addon instance %s deleted successfully\n", instanceName)
		},
	}

	return cmd
}
