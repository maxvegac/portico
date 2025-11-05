package embed

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ExtractStaticFiles extracts all embedded static files to the filesystem
func ExtractStaticFiles(targetDir string) error {
	// Extract static files
	staticFiles := []string{
		"static/reverse-proxy/Caddyfile",
		"static/config.yml",
		"static/reverse-proxy/docker-compose.yml",
		"static/www/index.html",
	}

	// Extract addon definitions
	addonDefs, err := fs.Glob(StaticFiles, "static/addons/definitions/*.yml")
	if err != nil {
		return fmt.Errorf("error listing addon definitions: %w", err)
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("error creating target directory: %w", err)
	}

	// Extract static files
	for _, file := range staticFiles {
		if err := extractFile(file, targetDir); err != nil {
			return err
		}
	}

	// Extract addon definitions
	for _, file := range addonDefs {
		if err := extractFile(file, targetDir); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from embed to target directory
func extractFile(embedPath string, targetDir string) error {
	// Read from embed
	content, err := StaticFiles.ReadFile(embedPath)
	if err != nil {
		return fmt.Errorf("error reading %s from embed: %w", embedPath, err)
	}

	// Determine target path
	// Remove "static/" prefix from embedPath
	relPath := embedPath
	if len(embedPath) > 7 && embedPath[:7] == "static/" {
		relPath = embedPath[7:]
	}

	targetPath := filepath.Join(targetDir, relPath)

	// Create target directory if needed
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", targetPath, err)
	}

	// Write file
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return fmt.Errorf("error writing %s: %w", targetPath, err)
	}

	return nil
}

// ExtractStaticFile extracts a single static file from embed
func ExtractStaticFile(embedPath, targetPath string) error {
	content, err := StaticFiles.ReadFile(embedPath)
	if err != nil {
		return fmt.Errorf("error reading %s from embed: %w", embedPath, err)
	}

	// Create target directory if needed
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", targetPath, err)
	}

	// Write file
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return fmt.Errorf("error writing %s: %w", targetPath, err)
	}

	return nil
}

// ExtractAddonDefinition extracts an addon definition from embed
func ExtractAddonDefinition(addonType, targetDir string) error {
	embedPath := fmt.Sprintf("static/addons/definitions/%s.yml", addonType)
	targetPath := filepath.Join(targetDir, addonType+".yml")
	return ExtractStaticFile(embedPath, targetPath)
}

// ExtractTemplate extracts a template file from embed to filesystem
func ExtractTemplate(templateName, targetPath string) error {
	embedPath := fmt.Sprintf("templates/%s", templateName)
	content, err := Templates.ReadFile(embedPath)
	if err != nil {
		return fmt.Errorf("error reading %s from embed: %w", embedPath, err)
	}

	// Create target directory if needed
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", targetPath, err)
	}

	// Write file
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return fmt.Errorf("error writing %s: %w", targetPath, err)
	}

	return nil
}
