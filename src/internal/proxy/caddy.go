package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	// Write to proxy directory.
	// os.WriteFile truncates in place (keeps the inode): the file is bind-mounted
	// into the Caddy container, so it must not be replaced via rename.
	if err := os.WriteFile(caddyfilePath, content, 0o644); err != nil {
		return fmt.Errorf("error writing Caddyfile: %w", err)
	}

	// Fix file ownership if running as root
	_ = util.FixFileOwnership(caddyfilePath)

	// Apply the new configuration (app Caddyfiles are imported by the main one).
	// A failed reload is not fatal: Caddy keeps serving the previous config.
	if err := cm.ReloadCaddy(); err != nil {
		fmt.Printf("⚠️  Warning: could not reload Caddy: %v\n", err)
	}

	return nil
}

// ReloadCaddy reloads the Caddy configuration inside the reverse-proxy container
func (cm *CaddyManager) ReloadCaddy() error {
	composeFile := filepath.Join(cm.ConfigDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("reverse-proxy docker-compose.yml not found in %s", cm.ConfigDir)
	}

	cmd := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", "caddy",
		"caddy", "reload", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile")
	cmd.Dir = cm.ConfigDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// GetCaddyfilePath returns the path to the Caddyfile
func (cm *CaddyManager) GetCaddyfilePath() string {
	return filepath.Join(cm.ConfigDir, "Caddyfile")
}
