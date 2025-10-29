package proxy

import (
	"fmt"
	"os"
	"path/filepath"
)

// CaddyManager handles Caddy proxy configuration
type CaddyManager struct {
	ConfigDir    string
	TemplatesDir string
}

// NewCaddyManager creates a new CaddyManager
func NewCaddyManager(configDir, templatesDir string) *CaddyManager {
	return &CaddyManager{
		ConfigDir:    configDir,
		TemplatesDir: templatesDir,
	}
}

// UpdateCaddyfile copies the static Caddyfile to the proxy directory
func (cm *CaddyManager) UpdateCaddyfile(appsDir string) error {
	caddyfilePath := filepath.Join(cm.ConfigDir, "Caddyfile")

	// Ensure directory exists
	if err := os.MkdirAll(cm.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("error creating proxy directory: %w", err)
	}

	// Copy static Caddyfile (which includes import /home/portico/apps/*/Caddyfile)
	staticCaddyfilePath := filepath.Join(cm.TemplatesDir, "..", "static", "Caddyfile")
	
	// Read static Caddyfile
	content, err := os.ReadFile(staticCaddyfilePath)
	if err != nil {
		return fmt.Errorf("error reading static Caddyfile: %w", err)
	}

	// Write to proxy directory
	if err := os.WriteFile(caddyfilePath, content, 0o644); err != nil {
		return fmt.Errorf("error writing Caddyfile: %w", err)
	}

	return nil
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
