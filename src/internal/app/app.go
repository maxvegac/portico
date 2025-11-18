package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/util"
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
	Replicas    int               `yaml:"replicas,omitempty"` // Number of instances (default: 1)
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
func (am *Manager) CreateAppDirectories(name string) error {
	appDir := filepath.Join(am.AppsDir, name)

	// Create app directory
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	// Create env directory (for secrets, but don't create default secrets)
	envDir := filepath.Join(appDir, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("error creating env directory: %w", err)
	}

	return nil
}

// CreateApp creates a new application (deprecated - kept for backwards compatibility)
// Now just creates directories and secrets
func (am *Manager) CreateApp(name string, port int) error {
	return am.CreateAppDirectories(name)
}

// SaveApp saves an application configuration to docker-compose.yml
func (am *Manager) SaveApp(app *App) error {
	appDir := filepath.Join(am.AppsDir, app.Name)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("docker-compose.yml not found for app %s: %w", app.Name, err)
	}

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

// LoadApp loads an application configuration from docker-compose.yml
func (am *Manager) LoadApp(name string) (*App, error) {
	appDir := filepath.Join(am.AppsDir, name)
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); err != nil {
		return nil, fmt.Errorf("docker-compose.yml not found for app %s: %w", name, err)
	}

	return am.LoadAppFromCompose(name)
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

	// Load template from filesystem first, then embedded files
	templateData, err := embed.LoadTemplate(am.TemplatesDir, "caddy-app.tmpl")
	if err != nil {
		return fmt.Errorf("error reading caddy-app template: %w", err)
	}

	t, err := template.New("caddy-app").Parse(string(templateData))
	if err != nil {
		return fmt.Errorf("error parsing caddy-app template: %w", err)
	}

	// Create output file
	file, err := os.Create(caddyfilePath)
	if err != nil {
		return fmt.Errorf("error creating Caddyfile: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Find the HTTP service by matching app.Port (http_port) with service port
	// This allows any service to be the HTTP service, not just "web"
	var mainService *Service
	var servicePort int

	if len(app.Services) == 0 {
		return fmt.Errorf("no services found in app %s", name)
	}

	// If app.Port is 0, this is a background worker - skip Caddyfile
	if app.Port == 0 {
		return fmt.Errorf("no HTTP port configured for app %s (background worker, no Caddyfile needed)", name)
	}

	// Find service with port matching app.Port (http_port)
	for i := range app.Services {
		if app.Services[i].Port == app.Port {
			mainService = &app.Services[i]
			servicePort = app.Port
			break
		}
	}

	// If no service found with matching port, try to find first service with a port > 0
	if mainService == nil {
		for i := range app.Services {
			if app.Services[i].Port > 0 {
				mainService = &app.Services[i]
				servicePort = app.Services[i].Port
				// Update app.Port to match the service port
				app.Port = servicePort
				break
			}
		}
	}

	// If still not found, this is a background worker
	if mainService == nil {
		return fmt.Errorf("no service with HTTP port %d found in app %s (background worker, no Caddyfile needed)", app.Port, name)
	}

	// Execute template
	if err := t.Execute(file, struct {
		AppName     string
		Domain      string
		ServiceName string
		Port        int
	}{
		AppName:     name,
		Domain:      domain,
		ServiceName: mainService.Name,
		Port:        servicePort,
	}); err != nil {
		return fmt.Errorf("error executing caddy-app template: %w", err)
	}

	// Close file explicitly before fixing ownership (defer will handle if this fails)
	if err := file.Close(); err != nil {
		return fmt.Errorf("error closing Caddyfile: %w", err)
	}

	// Fix file ownership if running as root
	_ = util.FixFileOwnership(caddyfilePath)

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
