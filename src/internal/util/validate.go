package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	// App names become directory names, Docker Compose project names and
	// subdomains, so they must be safe for all three.
	appNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]{0,61}[a-z0-9])?$`)
	// Service names form "<app>-<service>" DNS aliases, so no dots.
	serviceNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)
	// Secrets are stored as files inside the app env/ directory.
	secretNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	// Database identifiers are interpolated into SQL/JS, so keep them strict.
	dbIdentifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)
)

// ValidateAppName checks that a name is safe to use as a path, compose project and subdomain.
func ValidateAppName(name string) error {
	if !appNameRe.MatchString(name) {
		return fmt.Errorf("invalid app name %q: use lowercase letters, digits, '-' or '_' (max 63 chars, must start and end with a letter or digit)", name)
	}
	return nil
}

// AddonPrefix namespaces addon compose projects and DNS aliases so they can't
// collide with apps ("<app>" projects and "<app>-<service>" aliases)
const AddonPrefix = "addon-"

// ValidateNewAppName validates the name of an app being created: on top of
// ValidateAppName it rejects names whose compose project or DNS aliases would
// collide with Portico's own (the reverse proxy and addon instances).
func ValidateNewAppName(name string) error {
	if err := ValidateAppName(name); err != nil {
		return err
	}
	if name == "reverse-proxy" || name == "portico" || name == "addon" || strings.HasPrefix(name, AddonPrefix) {
		return fmt.Errorf("app name %q is reserved", name)
	}
	return nil
}

// ValidateServiceName checks that a service name is valid for Docker Compose.
func ValidateServiceName(name string) error {
	if !serviceNameRe.MatchString(name) {
		return fmt.Errorf("invalid service name %q: use letters, digits, '-' or '_' (must start with a letter or digit)", name)
	}
	return nil
}

// ValidateSecretName checks that a secret name is a plain file name (no path components).
func ValidateSecretName(name string) error {
	if !secretNameRe.MatchString(name) || name == "." || name == ".." {
		return fmt.Errorf("invalid secret name %q: use letters, digits, '-', '_' or '.' (no slashes)", name)
	}
	return nil
}

// ValidateDBIdentifier checks that a database name is safe to interpolate into queries.
func ValidateDBIdentifier(name string) error {
	if !dbIdentifierRe.MatchString(name) {
		return fmt.Errorf("invalid database name %q: use letters, digits and '_' (must start with a letter or '_', max 63 chars)", name)
	}
	return nil
}

// RandomSecret returns a cryptographically random hex string of n bytes (2n characters).
func RandomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error generating random secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
