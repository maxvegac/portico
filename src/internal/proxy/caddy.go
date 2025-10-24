package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// CaddyManager handles Caddy proxy configuration
type CaddyManager struct {
	ConfigDir string
}

// NewCaddyManager creates a new CaddyManager
func NewCaddyManager(configDir string) *CaddyManager {
	return &CaddyManager{
		ConfigDir: configDir,
	}
}

// UpdateCaddyfile updates the Caddyfile with current applications
func (cm *CaddyManager) UpdateCaddyfile(appsDir string) error {
	caddyfilePath := filepath.Join(cm.ConfigDir, "Caddyfile")

	// Ensure directory exists
	if err := os.MkdirAll(cm.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("error creating proxy directory: %w", err)
	}

	// Get all app directories
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("error reading apps directory: %w", err)
	}

	var apps []AppConfig
	for _, entry := range entries {
		if entry.IsDir() {
			appName := entry.Name()
			caddyConfPath := filepath.Join(appsDir, appName, "caddy.conf")

			// Check if caddy.conf exists
			if _, statErr := os.Stat(caddyConfPath); statErr == nil {
				apps = append(apps, AppConfig{
					Name:          appName,
					CaddyConfPath: caddyConfPath,
				})
			}
		}
	}

	// Load template
	templatePath := "templates/caddyfile.tmpl"
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing caddyfile template: %w", err)
	}

	// Create output file
	file, err := os.Create(caddyfilePath)
	if err != nil {
		return fmt.Errorf("error creating caddyfile: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := t.Execute(file, struct {
		Apps []AppConfig
	}{
		Apps: apps,
	}); err != nil {
		return fmt.Errorf("error executing caddyfile template: %w", err)
	}

	return nil
}

// AppConfig represents application configuration for Caddy
type AppConfig struct {
	Name          string
	Domain        string
	Port          int
	CaddyConfPath string
}

// ReloadCaddy reloads the Caddy configuration
func (cm *CaddyManager) ReloadCaddy() error {
	// This would typically send a signal to Caddy to reload
	// For now, we'll just return success
	// In production, you might use: systemctl reload caddy
	return nil
}

// GetCaddyfilePath returns the path to the Caddyfile
func (cm *CaddyManager) GetCaddyfilePath() string {
	return filepath.Join(cm.ConfigDir, "Caddyfile")
}
