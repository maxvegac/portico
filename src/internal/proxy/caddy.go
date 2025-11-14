package proxy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/util"
)

// CaddyManager handles Caddy proxy configuration
type CaddyManager struct {
	ConfigDir string
}

// NewCaddyManager creates a new CaddyManager
func NewCaddyManager(configDir, _ string) *CaddyManager {
	return &CaddyManager{
		ConfigDir: configDir,
	}
}

// UpdateCaddyfile copies the static Caddyfile to the proxy directory
func (cm *CaddyManager) UpdateCaddyfile(appsDir string) error {
	caddyfilePath := filepath.Join(cm.ConfigDir, "Caddyfile")

	// Ensure directory exists
	if err := os.MkdirAll(cm.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("error creating proxy directory: %w", err)
	}

	// Read static Caddyfile from embedded files
	content, err := embed.StaticFiles.ReadFile("static/reverse-proxy/Caddyfile")
	if err != nil {
		return fmt.Errorf("error reading static Caddyfile from embed: %w", err)
	}

	// Write to proxy directory
	if err := os.WriteFile(caddyfilePath, content, 0o644); err != nil {
		return fmt.Errorf("error writing Caddyfile: %w", err)
	}

	// Fix file ownership if running as root
	_ = util.FixFileOwnership(caddyfilePath)

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
