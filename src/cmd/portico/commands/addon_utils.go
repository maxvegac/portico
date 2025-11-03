package commands

import (
	"strings"
)

// generateSecret generates a default secret value
func generateSecret(secretName string) string {
	nameLower := strings.ToLower(secretName)
	if strings.Contains(nameLower, "password") {
		return "changeme123"
	}
	if strings.Contains(nameLower, "user") {
		return "admin"
	}
	if strings.Contains(nameLower, "name") || strings.Contains(nameLower, "db") {
		return "database"
	}
	if strings.Contains(nameLower, "root") {
		return "root"
	}
	return "default"
}
