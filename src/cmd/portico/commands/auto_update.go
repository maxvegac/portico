package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// NewCheckUpdateCmd creates a command to check for updates without downloading
func NewCheckUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-update",
		Short: "Check if a newer version is available",
		Long:  `Check if a newer version of Portico is available without downloading it.`,
		Run: func(cmd *cobra.Command, _ []string) {
			// Determine if we're in dev mode
			isDev := false
			if devFlag, _ := cmd.Flags().GetBool("dev"); devFlag {
				isDev = true
			}

			// Create update manager
			updateManager := NewUpdateManager("maxvegac", "portico", isDev)

			// Get current version
			currentVersion, err := updateManager.GetCurrentVersion()
			if err != nil {
				fmt.Printf("Error getting current version: %v\n", err)
				return
			}

			// Check for updates
			latestRelease, err := updateManager.CheckForUpdates()
			if err != nil {
				fmt.Printf("Error checking for updates: %v\n", err)
				return
			}

			// Compare versions
			if latestRelease.TagName == currentVersion {
				fmt.Printf("✅ You're running the latest version: %s\n", currentVersion)
			} else {
				fmt.Printf("📦 Update available!\n")
				fmt.Printf("   Current: %s\n", currentVersion)
				fmt.Printf("   Latest:  %s\n", latestRelease.TagName)
				fmt.Printf("   Run 'portico update' to update\n")
			}
		},
	}
}

// NewAutoUpdateCmd creates a command for automatic background updates
func NewAutoUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto-update",
		Short: "Enable or disable automatic updates",
		Long:  `Configure automatic updates for Portico. When enabled, Portico will check for updates periodically.`,
		Run: func(cmd *cobra.Command, _ []string) {
			enable, _ := cmd.Flags().GetBool("enable")
			disable, _ := cmd.Flags().GetBool("disable")
			status, _ := cmd.Flags().GetBool("status")

			if status {
				checkAutoUpdateStatus()
				return
			}

			switch {
			case enable:
				enableAutoUpdate()
			case disable:
				disableAutoUpdate()
			default:
				_ = cmd.Help()
			}
		},
	}
}

// checkAutoUpdateStatus checks if auto-update is enabled
func checkAutoUpdateStatus() {
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "auto-update")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("Auto-update: Disabled")
	} else {
		fmt.Println("Auto-update: Enabled")
	}
}

// enableAutoUpdate enables automatic updates
func enableAutoUpdate() {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		fmt.Printf("Error creating config directory: %v\n", err)
		return
	}

	configFile := filepath.Join(configDir, "auto-update")
	if err := os.WriteFile(configFile, []byte(time.Now().Format(time.RFC3339)), 0o600); err != nil {
		fmt.Printf("Error enabling auto-update: %v\n", err)
		return
	}

	fmt.Println("✅ Auto-update enabled")
	fmt.Println("Portico will check for updates every time you run a command")
}

// disableAutoUpdate disables automatic updates
func disableAutoUpdate() {
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "auto-update")

	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error disabling auto-update: %v\n", err)
		return
	}

	fmt.Println("✅ Auto-update disabled")
}

// getConfigDir returns the configuration directory
func getConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".portico"
	}
	return filepath.Join(homeDir, ".portico")
}

// CheckAutoUpdate checks for updates if auto-update is enabled
func CheckAutoUpdate() {
	configDir := getConfigDir()
	configFile := filepath.Join(configDir, "auto-update")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return // Auto-update not enabled
	}

	// Check if we should skip this check (avoid checking too frequently)
	lastCheckFile := filepath.Join(configDir, "last-check")
	if lastCheck, err := os.ReadFile(lastCheckFile); err == nil {
		if lastCheckTime, err := time.Parse(time.RFC3339, string(lastCheck)); err == nil {
			if time.Since(lastCheckTime) < 24*time.Hour {
				return // Checked within last 24 hours
			}
		}
	}

	// Update last check time
	if err := os.WriteFile(lastCheckFile, []byte(time.Now().Format(time.RFC3339)), 0o600); err != nil {
		return
	}

	// Check for updates
	updateManager := NewUpdateManager("maxvegac", "portico", false)
	currentVersion, err := updateManager.GetCurrentVersion()
	if err != nil {
		return
	}

	latestRelease, err := updateManager.CheckForUpdates()
	if err != nil {
		return
	}

	if latestRelease.TagName != currentVersion {
		fmt.Printf("🔄 Update available: %s -> %s (run 'portico update' to update)\n", currentVersion, latestRelease.TagName)
	}
}
