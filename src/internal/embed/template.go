package embed

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadTemplate loads a template file, trying filesystem first, then embedded files
func LoadTemplate(templatesDir, templateName string) ([]byte, error) {
	// Try filesystem first
	templatePath := filepath.Join(templatesDir, templateName)
	if data, err := os.ReadFile(templatePath); err == nil {
		return data, nil
	}

	// Fallback to embedded files
	embedPath := fmt.Sprintf("templates/%s", templateName)
	data, err := Templates.ReadFile(embedPath)
	if err != nil {
		return nil, fmt.Errorf("template %s not found in filesystem (%s) or embedded files (%s): %w", templateName, templatePath, embedPath, err)
	}

	return data, nil
}
