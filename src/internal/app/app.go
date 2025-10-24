package app

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

// App represents a Portico application
type App struct {
	Name        string            `yaml:"name"`
	Domain      string            `yaml:"domain"`
	Port        int               `yaml:"port"`
	Environment map[string]string `yaml:"environment"`
	Services    []Service         `yaml:"services"`
}

// Service represents a service within an application
type Service struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Port        int               `yaml:"port"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
	Secrets     []string          `yaml:"secrets"`
	DependsOn   []string          `yaml:"depends_on"`
}

// AppManager handles application operations
type Manager struct {
	AppsDir string
}

// NewManager creates a new Manager
func NewManager(appsDir string) *Manager {
	return &Manager{
		AppsDir: appsDir,
	}
}

// CreateApp creates a new application
func (am *Manager) CreateApp(name string) error {
	appDir := filepath.Join(am.AppsDir, name)

	// Create app directory
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	// Create env directory
	envDir := filepath.Join(appDir, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("error creating env directory: %w", err)
	}

	// Create default app.yml
	app := &App{
		Name:        name,
		Domain:      fmt.Sprintf("%s.localhost", name),
		Port:        8080,
		Environment: make(map[string]string),
		Services: []Service{
			{
				Name:  "api",
				Image: "node:22-alpine",
				Port:  3000,
				Environment: map[string]string{
					"NODE_ENV": "production",
					"PORT":     "3000",
				},
			},
		},
	}

	if err := am.SaveApp(app); err != nil {
		return err
	}

	// Create default caddy.conf
	if err := am.CreateDefaultCaddyConf(name); err != nil {
		return err
	}

	// Create default secret files
	return am.CreateDefaultSecrets(name)
}

// SaveApp saves an application configuration
func (am *Manager) SaveApp(app *App) error {
	appDir := filepath.Join(am.AppsDir, app.Name)
	appFile := filepath.Join(appDir, "app.yml")

	data, err := yaml.Marshal(app)
	if err != nil {
		return fmt.Errorf("error marshaling app config: %w", err)
	}

	return os.WriteFile(appFile, data, 0o600)
}

// LoadApp loads an application configuration
func (am *Manager) LoadApp(name string) (*App, error) {
	appDir := filepath.Join(am.AppsDir, name)
	appFile := filepath.Join(appDir, "app.yml")

	data, err := os.ReadFile(appFile)
	if err != nil {
		return nil, fmt.Errorf("error reading app config: %w", err)
	}

	var app App
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("error unmarshaling app config: %w", err)
	}

	return &app, nil
}

// ListApps returns a list of all applications
func (am *Manager) ListApps() ([]string, error) {
	entries, err := os.ReadDir(am.AppsDir)
	if err != nil {
		return nil, fmt.Errorf("error reading apps directory: %w", err)
	}

	var apps []string
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, entry.Name())
		}
	}

	return apps, nil
}

// DeleteApp deletes an application
func (am *Manager) DeleteApp(name string) error {
	appDir := filepath.Join(am.AppsDir, name)
	return os.RemoveAll(appDir)
}

// CreateDefaultCaddyConf creates a default caddy.conf file for an application
func (am *Manager) CreateDefaultCaddyConf(name string) error {
	appDir := filepath.Join(am.AppsDir, name)
	caddyConfPath := filepath.Join(appDir, "caddy.conf")

	domain := fmt.Sprintf("%s.localhost", name)

	// Load template
	templatePath := "templates/caddy-app.tmpl"
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing caddy-app template: %w", err)
	}

	// Create output file
	file, err := os.Create(caddyConfPath)
	if err != nil {
		return fmt.Errorf("error creating caddy.conf: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := t.Execute(file, struct {
		AppName string
		Domain  string
	}{
		AppName: name,
		Domain:  domain,
	}); err != nil {
		return fmt.Errorf("error executing caddy-app template: %w", err)
	}

	return nil
}

// CreateDefaultSecrets creates default secret files for an application
func (am *Manager) CreateDefaultSecrets(name string) error {
	appDir := filepath.Join(am.AppsDir, name)
	envDir := filepath.Join(appDir, "env")

	// Create default secret files
	secrets := map[string]string{
		"database_password": "changeme123",
		"api_key":           "sk-1234567890abcdef",
		"jwt_secret":        "jwt-secret-key-very-long-and-secure",
	}

	for secretName, defaultValue := range secrets {
		secretPath := filepath.Join(envDir, secretName)
		if err := os.WriteFile(secretPath, []byte(defaultValue), 0o600); err != nil {
			return fmt.Errorf("error creating secret %s: %w", secretName, err)
		}
	}

	return nil
}
