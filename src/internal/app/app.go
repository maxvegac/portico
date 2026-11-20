package app

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/util"
)

// DefaultHTTPPort is the internal container port Caddy proxies to when none is configured
const DefaultHTTPPort = 8080

// App represents a Portico application
type App struct {
	Name        string `yaml:"name"`
	Domain      string `yaml:"domain"`
	Port        int    `yaml:"port"`
	HttpService string `yaml:"http_service"` // Service Caddy proxies to ("" = auto)
	// DisabledHTTPPort is the port to restore when HTTP is re-enabled (Port is 0 while disabled)
	DisabledHTTPPort int               `yaml:"-"`
	Environment      map[string]string `yaml:"environment"`
	Services         []Service         `yaml:"services"`
}

// FindService returns a pointer to the service with the given name, or nil
func (a *App) FindService(name string) *Service {
	for i := range a.Services {
		if a.Services[i].Name == name {
			return &a.Services[i]
		}
	}
	return nil
}

// HTTPServiceName returns the service that receives HTTP traffic: the explicitly
// configured one, otherwise "web", otherwise the only/first service (sorted by name).
func (a *App) HTTPServiceName() string {
	if a.HttpService != "" && a.FindService(a.HttpService) != nil {
		return a.HttpService
	}
	if a.FindService("web") != nil {
		return "web"
	}
	if len(a.Services) > 0 {
		return a.Services[0].Name
	}
	return ""
}

// DockerServices converts the app services to docker services (replicas default to 1)
func (a *App) DockerServices() []docker.Service {
	var dockerServices []docker.Service
	for _, svc := range a.Services {
		replicas := svc.Replicas
		if replicas == 0 {
			replicas = 1
		}
		dockerServices = append(dockerServices, docker.Service{
			Name:        svc.Name,
			Image:       svc.Image,
			Port:        svc.Port,
			ExtraPorts:  svc.ExtraPorts,
			Environment: svc.Environment,
			Volumes:     svc.Volumes,
			Secrets:     svc.Secrets,
			DependsOn:   svc.DependsOn,
			Replicas:    replicas,
		})
	}
	return dockerServices
}

// Metadata returns the x-portico metadata for the app
func (a *App) Metadata() *docker.PorticoMetadata {
	return &docker.PorticoMetadata{
		Domain:       a.Domain,
		Port:         a.Port,
		HttpEnabled:  a.Port > 0,
		HttpService:  a.HttpService,
		DisabledPort: a.DisabledHTTPPort,
	}
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

// AppDir returns the directory of an app after validating its name
func (am *Manager) AppDir(name string) (string, error) {
	if err := util.ValidateAppName(name); err != nil {
		return "", err
	}
	return filepath.Join(am.AppsDir, name), nil
}

// Exists reports whether an app has been created (has a docker-compose.yml)
func (am *Manager) Exists(name string) bool {
	appDir, err := am.AppDir(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(appDir, "docker-compose.yml"))
	return err == nil
}

// CreateAppDirectories creates app directory structure and default secrets
func (am *Manager) CreateAppDirectories(name string) error {
	appDir, err := am.AppDir(name)
	if err != nil {
		return err
	}

	// Create app directory
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	// Create env directory (for secrets, but don't create default secrets)
	envDir := filepath.Join(appDir, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("error creating env directory: %w", err)
	}

	// When run as root, hand the directories to the portico user (git hooks run as portico)
	_ = util.FixDirOwnership(appDir)

	return nil
}

// CreateApp creates a new application (deprecated - kept for backwards compatibility)
// Now just creates directories and secrets
func (am *Manager) CreateApp(name string, port int) error {
	return am.CreateAppDirectories(name)
}

// SaveApp saves an application configuration to docker-compose.yml
func (am *Manager) SaveApp(app *App) error {
	appDir, err := am.AppDir(app.Name)
	if err != nil {
		return err
	}
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("docker-compose.yml not found for app %s: %w", app.Name, err)
	}

	// Use docker manager to update compose file
	dm := docker.NewManager("") // Registry URL not needed for updates

	return dm.GenerateDockerCompose(appDir, app.DockerServices(), app.Metadata())
}

// LoadApp loads an application configuration from docker-compose.yml
func (am *Manager) LoadApp(name string) (*App, error) {
	appDir, err := am.AppDir(name)
	if err != nil {
		return nil, err
	}
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); err != nil {
		return nil, fmt.Errorf("docker-compose.yml not found for app %s: %w", name, err)
	}

	return am.LoadAppFromCompose(name)
}

// LoadAppFromCompose loads application configuration from docker-compose.yml
func (am *Manager) LoadAppFromCompose(name string) (*App, error) {
	appDir, err := am.AppDir(name)
	if err != nil {
		return nil, err
	}

	// Use docker manager to load compose file
	dm := docker.NewManager("") // Registry URL not needed for loading
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return nil, fmt.Errorf("error loading docker-compose.yml: %w", err)
	}

	// Extract metadata from x-portico
	domain := ""
	port := 0
	httpEnabled := false
	httpService := ""
	disabledPort := 0
	var replicas map[string]int
	if compose.XPortico != nil {
		domain = compose.XPortico.Domain
		httpEnabled = compose.XPortico.HttpEnabled
		if httpEnabled {
			port = compose.XPortico.Port
		}
		httpService = compose.XPortico.HttpService
		disabledPort = compose.XPortico.DisabledPort
		replicas = compose.XPortico.Replicas
	}

	// Convert services from docker-compose.yml format to App.Service format
	var services []Service
	for svcName, svcData := range compose.Services {
		svc, err := convertServiceFromCompose(svcName, svcData)
		if err != nil {
			return nil, fmt.Errorf("error converting service %s: %w", svcName, err)
		}
		svc.Replicas = replicas[svcName]
		services = append(services, *svc)
	}
	// Map iteration order is random; keep services in a stable order
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	// Only generate domain based on IP if no domain is defined
	// If domain is already set (even if .localhost), preserve it
	if domain == "" {
		generated, err := defaultDomain(name)
		if err != nil {
			return nil, err
		}
		domain = generated
	}

	// Port is only set if HTTP is enabled
	// If http_enabled is false, port remains 0

	return &App{
		Name:             name,
		Domain:           domain,
		Port:             port, // HTTP port (0 if HTTP disabled)
		HttpService:      httpService,
		DisabledHTTPPort: disabledPort,
		Environment:      make(map[string]string), // App-level environment not stored in compose
		Services:         services,
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

	// Extract published ports. Every mapping is kept in ExtraPorts (the generator
	// only writes ExtraPorts). The HTTP port lives in x-portico, not here.
	if ports, ok := svcMap["ports"].([]interface{}); ok {
		for _, p := range ports {
			switch v := p.(type) {
			case string:
				svc.ExtraPorts = append(svc.ExtraPorts, v)
			case int:
				svc.ExtraPorts = append(svc.ExtraPorts, strconv.Itoa(v))
			}
		}
	}

	// Extract environment variables (map form: KEY: value)
	if env, ok := svcMap["environment"].(map[string]interface{}); ok {
		for k, v := range env {
			if v == nil {
				svc.Environment[k] = ""
			} else {
				svc.Environment[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	// Extract environment variables (list form: - KEY=value)
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
			if ok && !docker.IsManagedVolume(volStr) { // Exclude secrets and logs mounts (added by Portico)
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
	appDir, err := am.AppDir(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(appDir)
}

// DetectCaddyfileChanges checks if Caddyfile was manually modified
// by comparing its current hash with the stored hash in comment
func (am *Manager) DetectCaddyfileChanges(name string) (bool, error) {
	appDir := filepath.Join(am.AppsDir, name)
	caddyfilePath := filepath.Join(appDir, "Caddyfile")

	// Read current Caddyfile
	currentData, err := os.ReadFile(caddyfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // No Caddyfile = not manually modified
		}
		return false, err
	}

	// Extract stored hash from comment
	hashRegex := regexp.MustCompile(`# Portico Generated - Hash: ([a-f0-9]+)`)
	matches := hashRegex.FindStringSubmatch(string(currentData))
	if len(matches) < 2 {
		// No hash found, likely manually created/modified
		return true, nil
	}
	storedHash := matches[1]

	// Remove hash comment for comparison
	contentWithoutHash := hashRegex.ReplaceAllString(string(currentData), "")

	// Calculate hash of current content (without hash comment)
	hash := sha256.Sum256([]byte(contentWithoutHash))
	currentHashStr := fmt.Sprintf("%x", hash)

	// If stored hash doesn't match current hash, file was manually modified
	return currentHashStr != storedHash, nil
}

// CreateDefaultCaddyfile creates a default Caddyfile for an application
// Reads directly from docker-compose.yml (single source of truth)
// Always regenerates the Caddyfile, but warns if manual changes are detected
func (am *Manager) CreateDefaultCaddyfile(name string) error {
	return am.CreateDefaultCaddyfileWithPrompt(name, false)
}

// CreateDefaultCaddyfileWithPrompt creates/updates Caddyfile with optional prompt for confirmation
func (am *Manager) CreateDefaultCaddyfileWithPrompt(name string, prompt bool) error {
	appDir, err := am.AppDir(name)
	if err != nil {
		return err
	}
	caddyfilePath := filepath.Join(appDir, "Caddyfile")

	// Check for manual changes if Caddyfile exists
	if _, err := os.Stat(caddyfilePath); err == nil {
		hasChanges, err := am.DetectCaddyfileChanges(name)
		if err != nil {
			return fmt.Errorf("error checking Caddyfile changes: %w", err)
		}
		if hasChanges && prompt {
			fmt.Println("⚠️  Warning: Caddyfile appears to have been manually modified.")
			fmt.Println("Portico will regenerate it, overwriting your changes.")
			fmt.Print("Continue? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(response)
			if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
				return ErrCancelled
			}
		}
	}

	// docker-compose.yml is the single source of truth
	a, err := am.LoadApp(name)
	if err != nil {
		return err
	}

	if a.Port == 0 {
		return fmt.Errorf("HTTP is not enabled for app %s (background worker, no Caddyfile needed)", name)
	}
	if len(a.Services) == 0 {
		return fmt.Errorf("no services found in app %s", name)
	}

	domain := a.Domain
	if strings.HasSuffix(domain, ".sslip.io") && domain == fmt.Sprintf("%s.sslip.io", name) {
		// Migrate old format appname.sslip.io (which does not resolve) to appname.IP.sslip.io
		if migrated, err := defaultDomain(name); err == nil {
			domain = migrated
			a.Domain = migrated
			if err := am.SaveApp(a); err != nil {
				return fmt.Errorf("error saving migrated domain: %w", err)
			}
		}
	}

	content, err := am.RenderCaddyfile(a.Name, domain, a.HTTPServiceName(), a.Port)
	if err != nil {
		return err
	}

	if err := util.WriteFileAtomic(caddyfilePath, content, 0o644); err != nil {
		return fmt.Errorf("error writing Caddyfile: %w", err)
	}

	// Fix file ownership if running as root
	_ = util.FixFileOwnership(caddyfilePath)

	return nil
}

// ErrCancelled is returned when the user declines a confirmation prompt
var ErrCancelled = fmt.Errorf("cancelled by user")

// caddyHashPlaceholder is substituted by the real hash after rendering
const caddyHashPlaceholder = "__PORTICO_HASH__"

// RenderCaddyfile renders the per-app Caddyfile, including the hash comment used
// by DetectCaddyfileChanges to detect manual edits.
func (am *Manager) RenderCaddyfile(appName, domain, serviceName string, port int) ([]byte, error) {
	templateData, err := embed.LoadTemplate(am.TemplatesDir, "caddy-app.tmpl")
	if err != nil {
		return nil, fmt.Errorf("error reading caddy-app template: %w", err)
	}

	t, err := template.New("caddy-app").Parse(string(templateData))
	if err != nil {
		return nil, fmt.Errorf("error parsing caddy-app template: %w", err)
	}

	templateVars := struct {
		AppName       string
		Domain        string
		ServiceName   string
		Upstream      string
		Port          int
		GeneratedHash string
	}{
		AppName:       appName,
		Domain:        domain,
		ServiceName:   serviceName,
		Upstream:      docker.ServiceAlias(appName, serviceName),
		Port:          port,
		GeneratedHash: caddyHashPlaceholder,
	}

	var buf strings.Builder
	if err := t.Execute(&buf, templateVars); err != nil {
		return nil, fmt.Errorf("error executing caddy-app template: %w", err)
	}
	rendered := buf.String()

	// Hash the content without the hash comment (same as DetectCaddyfileChanges does)
	withoutHash := strings.Replace(rendered, "# Portico Generated - Hash: "+caddyHashPlaceholder, "", 1)
	sum := sha256.Sum256([]byte(withoutHash))
	return []byte(strings.Replace(rendered, caddyHashPlaceholder, fmt.Sprintf("%x", sum), 1)), nil
}

// defaultDomain returns appname.IP.sslip.io using the configured or detected server IP
func defaultDomain(name string) (string, error) {
	configuredIP := ""
	if cfg, err := config.LoadConfig(); err == nil {
		configuredIP = cfg.ExternalIP
	}
	serverIP, err := util.GetServerIPWithFallback(configuredIP)
	if err != nil {
		return "", fmt.Errorf("failed to detect server IP for domain generation: %w", err)
	}
	if serverIP == "" {
		return "", fmt.Errorf("failed to detect server IP for domain generation")
	}
	return util.AppNameToSSlipIO(name, serverIP), nil
}

// DefaultDomain returns the default sslip.io domain for an app
func DefaultDomain(name string) (string, error) {
	return defaultDomain(name)
}
