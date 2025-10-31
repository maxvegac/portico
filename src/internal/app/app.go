package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/docker"
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
	ExtraPorts  []string          `yaml:"extra_ports"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
	Secrets     []string          `yaml:"secrets"`
	DependsOn   []string          `yaml:"depends_on"`
}

// AppManager handles application operations
type Manager struct {
	AppsDir      string
	TemplatesDir string
}

// NewManager creates a new Manager
func NewManager(appsDir, templatesDir string) *Manager {
	return &Manager{
		AppsDir:      appsDir,
		TemplatesDir: templatesDir,
	}
}

// CreateAppDirectories creates app directory structure and default secrets
// Does not create app.yml - that is now optional/legacy
func (am *Manager) CreateAppDirectories(name string) error {
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

	// Create default secret files
	return am.CreateDefaultSecrets(name)
}

// CreateApp creates a new application (deprecated - kept for backwards compatibility)
// Now just creates directories and secrets, app.yml is optional
func (am *Manager) CreateApp(name string, port int) error {
	return am.CreateAppDirectories(name)
}

// SaveApp saves an application configuration
// If docker-compose.yml exists, updates it. Otherwise saves to app.yml (legacy)
func (am *Manager) SaveApp(app *App) error {
	appDir := filepath.Join(am.AppsDir, app.Name)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// If docker-compose.yml exists, update it instead of app.yml
	if _, err := os.Stat(composeFile); err == nil {
		// Use docker manager to update compose file
		dm := docker.NewManager("") // Registry URL not needed for updates

		// Convert app services to docker services
		var dockerServices []docker.Service
		for _, svc := range app.Services {
			dockerServices = append(dockerServices, docker.Service{
				Name:        svc.Name,
				Image:       svc.Image,
				Port:        svc.Port,
				ExtraPorts:  svc.ExtraPorts,
				Environment: svc.Environment,
				Volumes:     svc.Volumes,
				Secrets:     svc.Secrets,
				DependsOn:   svc.DependsOn,
			})
		}

		// Update metadata
		metadata := &docker.PorticoMetadata{
			Domain: app.Domain,
			Port:   app.Port,
		}

		return dm.GenerateDockerCompose(appDir, dockerServices, metadata)
	}

	// Fallback: save to app.yml for backwards compatibility
	appFile := filepath.Join(appDir, "app.yml")
	data, err := yaml.Marshal(app)
	if err != nil {
		return fmt.Errorf("error marshaling app config: %w", err)
	}

	return os.WriteFile(appFile, data, 0o600)
}

// LoadApp loads an application configuration
// First tries to load from docker-compose.yml, falls back to app.yml if not found
func (am *Manager) LoadApp(name string) (*App, error) {
	appDir := filepath.Join(am.AppsDir, name)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Try to load from docker-compose.yml first
	if _, err := os.Stat(composeFile); err == nil {
		return am.LoadAppFromCompose(name)
	}

	// Fallback to app.yml for backwards compatibility
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

// LoadAppFromCompose loads application configuration from docker-compose.yml
func (am *Manager) LoadAppFromCompose(name string) (*App, error) {
	appDir := filepath.Join(am.AppsDir, name)

	// Use docker manager to load compose file
	dm := docker.NewManager("") // Registry URL not needed for loading
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return nil, fmt.Errorf("error loading docker-compose.yml: %w", err)
	}

	// Extract metadata from x-portico
	domain := ""
	port := 0
	if compose.XPortico != nil {
		domain = compose.XPortico.Domain
		port = compose.XPortico.Port
	}

	// Convert services from docker-compose.yml format to App.Service format
	var services []Service
	for svcName, svcData := range compose.Services {
		svc, err := convertServiceFromCompose(svcName, svcData)
		if err != nil {
			return nil, fmt.Errorf("error converting service %s: %w", svcName, err)
		}
		services = append(services, *svc)
	}

	// If domain/port not in metadata, try to extract from app name or defaults
	if domain == "" {
		domain = fmt.Sprintf("%s.localhost", name)
	}
	if port == 0 {
		port = 8080
	}

	return &App{
		Name:        name,
		Domain:      domain,
		Port:        port,
		Environment: make(map[string]string), // App-level environment not stored in compose
		Services:    services,
	}, nil
}

// convertServiceFromCompose converts a service from docker-compose.yml format to App.Service
func convertServiceFromCompose(name string, svcData interface{}) (*Service, error) {
	svcMap, ok := svcData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("service data is not a map")
	}

	svc := &Service{
		Name:        name,
		ExtraPorts:  []string{},
		Environment: make(map[string]string),
		Volumes:     []string{},
		Secrets:     []string{},
		DependsOn:   []string{},
	}

	// Extract image
	if img, ok := svcMap["image"].(string); ok {
		svc.Image = img
	}

	// Extract ports - primary port and extra ports
	if ports, ok := svcMap["ports"].([]interface{}); ok {
		primaryPort := 0
		for _, p := range ports {
			portStr, ok := p.(string)
			if !ok {
				continue
			}
			// Parse port mapping "host:container" or just "port"
			parts := strings.Split(portStr, ":")
			if len(parts) == 2 {
				containerPort, err := strconv.Atoi(parts[1])
				if err == nil {
					if primaryPort == 0 {
						primaryPort = containerPort
					} else {
						svc.ExtraPorts = append(svc.ExtraPorts, portStr)
					}
				}
			} else if len(parts) == 1 {
				port, err := strconv.Atoi(parts[0])
				if err == nil && primaryPort == 0 {
					primaryPort = port
				}
			}
		}
		svc.Port = primaryPort
	}

	// Extract environment variables
	if env, ok := svcMap["environment"].([]interface{}); ok {
		for _, e := range env {
			envStr, ok := e.(string)
			if !ok {
				continue
			}
			// Parse "KEY=VALUE" format
			parts := strings.SplitN(envStr, "=", 2)
			if len(parts) == 2 {
				svc.Environment[parts[0]] = parts[1]
			}
		}
	}

	// Extract volumes
	if volumes, ok := svcMap["volumes"].([]interface{}); ok {
		for _, v := range volumes {
			volStr, ok := v.(string)
			if ok && !strings.Contains(volStr, "/run/secrets") { // Exclude secrets mount
				svc.Volumes = append(svc.Volumes, volStr)
			}
		}
	}

	// Extract secrets
	if secrets, ok := svcMap["secrets"].([]interface{}); ok {
		for _, s := range secrets {
			if secretStr, ok := s.(string); ok {
				svc.Secrets = append(svc.Secrets, secretStr)
			}
		}
	}

	// Extract depends_on
	if depends, ok := svcMap["depends_on"].([]interface{}); ok {
		for _, d := range depends {
			if depStr, ok := d.(string); ok {
				svc.DependsOn = append(svc.DependsOn, depStr)
			}
		}
	}

	return svc, nil
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

// CreateDefaultCaddyfile creates a default Caddyfile for an application
func (am *Manager) CreateDefaultCaddyfile(name string) error {
	appDir := filepath.Join(am.AppsDir, name)
	caddyfilePath := filepath.Join(appDir, "Caddyfile")

	// Load app configuration to get the domain
	app, err := am.LoadApp(name)
	if err != nil {
		return fmt.Errorf("error loading app configuration: %w", err)
	}

	// Use domain from app configuration, fallback to default if empty
	domain := app.Domain
	if domain == "" {
		domain = fmt.Sprintf("%s.localhost", name)
	}

	// Load template from configured templates directory
	templatePath := filepath.Join(am.TemplatesDir, "caddy-app.tmpl")
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing caddy-app template: %w", err)
	}

	// Create output file
	file, err := os.Create(caddyfilePath)
	if err != nil {
		return fmt.Errorf("error creating Caddyfile: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Use port from app configuration, fallback to default if 0
	port := app.Port
	if port == 0 {
		port = 8080
	}

	// Execute template
	if err := t.Execute(file, struct {
		AppName string
		Domain  string
		Port    int
	}{
		AppName: name,
		Domain:  domain,
		Port:    port,
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
