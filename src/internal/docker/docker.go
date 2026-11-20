package docker

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/util"
)

// Manager handles Docker operations
type Manager struct {
	RegistryURL string
}

// NewManager creates a new Manager
func NewManager(registryURL string) *Manager {
	return &Manager{
		RegistryURL: registryURL,
	}
}

// DeployApp deploys an application using docker compose
// If services have replicas > 1, uses --scale to scale them
func (dm *Manager) DeployApp(appDir string, services []Service) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found in %s", appDir)
	}

	// Extract app name from directory to ensure consistent project naming
	// Docker Compose uses project name as prefix for service names (e.g., myapp-web)
	appName := filepath.Base(appDir)

	// Ensure portico-network exists before deploying
	if err := dm.ensureNetworkExists("portico-network"); err != nil {
		return fmt.Errorf("error ensuring portico-network exists: %w", err)
	}

	// Create the per-service logs directories ourselves: if Docker creates a
	// missing bind mount source it is owned by root, and neither portico nor a
	// non-root container user could write to it. (Best effort.)
	for _, svc := range services {
		logsDir := filepath.Join(logsMountPrefix, appName, svc.Name)
		if err := os.MkdirAll(logsDir, 0o755); err == nil {
			_ = util.FixDirOwnership(logsDir)
		}
	}

	// Build docker compose command with explicit project name
	// This ensures services are named consistently: appname-servicename
	// --remove-orphans stops containers of services that were removed from the compose file.
	// Skip it if an addon instance from an older version shares this compose project
	// name: its database container would be treated as an orphan.
	args := []string{"compose", "-f", composeFile, "-p", appName, "up", "-d"}
	if !legacyAddonProject(appName) {
		args = append(args, "--remove-orphans")
	}

	// Add --scale flags for services with replicas > 1
	for _, svc := range services {
		if svc.Replicas > 1 {
			args = append(args, "--scale", fmt.Sprintf("%s=%d", svc.Name, svc.Replicas))
		}
	}

	// Run docker compose up
	cmd := exec.Command("docker", args...)
	cmd.Dir = appDir

	output, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		return fmt.Errorf("error running docker compose: %s\n%s", cmdErr, string(output))
	}

	return nil
}

// StopApp stops an application
func (dm *Manager) StopApp(appDir string) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found in %s", appDir)
	}

	// Extract app name from directory for consistent project naming
	appName := filepath.Base(appDir)

	cmd := exec.Command("docker", "compose", "-f", composeFile, "-p", appName, "down")
	cmd.Dir = appDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error stopping application: %s\n%s", err, string(output))
	}

	return nil
}

// RestartApp restarts all services in an application
func (dm *Manager) RestartApp(appDir string) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found in %s", appDir)
	}

	// Extract app name from directory for consistent project naming
	appName := filepath.Base(appDir)

	cmd := exec.Command("docker", "compose", "-f", composeFile, "-p", appName, "restart")
	cmd.Dir = appDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error restarting services: %s\n%s", err, string(output))
	}

	return nil
}

// RestartService restarts a specific service in an application
func (dm *Manager) RestartService(appDir string, serviceName string) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found in %s", appDir)
	}

	// Extract app name from directory for consistent project naming
	appName := filepath.Base(appDir)

	cmd := exec.Command("docker", "compose", "-f", composeFile, "-p", appName, "restart", serviceName)
	cmd.Dir = appDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error restarting service %s: %s\n%s", serviceName, err, string(output))
	}

	return nil
}

// ComposeFile represents a docker-compose.yml structure with Portico metadata
type ComposeFile struct {
	Name     string                 `yaml:"name,omitempty"` // Project name
	Services map[string]interface{} `yaml:"services"`
	Networks map[string]interface{} `yaml:"networks,omitempty"`
	Secrets  map[string]interface{} `yaml:"secrets,omitempty"`
	XPortico *PorticoMetadata       `yaml:"x-portico,omitempty"`
	// Other top-level keys (volumes, configs, x-*, ...) are kept as-is
	Extra map[string]interface{} `yaml:",inline"`
}

// composeHash returns the hash of a compose file, computed without the hash field itself
func composeHash(c *ComposeFile) (string, error) {
	var saved string
	if c.XPortico != nil {
		saved = c.XPortico.Generated
		c.XPortico.Generated = ""
		defer func() { c.XPortico.Generated = saved }()
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// AcceptManualChanges stores the hash of the current docker-compose.yml so that
// manual edits are no longer reported as unexpected changes.
func (dm *Manager) AcceptManualChanges(appDir string) error {
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return err
	}
	if compose.XPortico == nil {
		compose.XPortico = &PorticoMetadata{}
	}
	hash, err := composeHash(compose)
	if err != nil {
		return err
	}
	compose.XPortico.Generated = hash
	data, err := yaml.Marshal(compose)
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(filepath.Join(appDir, "docker-compose.yml"), data, 0o644)
}

// PorticoMetadata stores Portico-specific configuration
type PorticoMetadata struct {
	Domain      string `yaml:"domain,omitempty"`
	Port        int    `yaml:"http_port,omitempty"`
	HttpEnabled bool   `yaml:"http_enabled,omitempty"`
	HttpService string `yaml:"http_service,omitempty"` // Service Caddy proxies to
	// HTTP port to restore when HTTP is re-enabled (only set while it is disabled)
	DisabledPort int            `yaml:"disabled_http_port,omitempty"`
	Replicas     map[string]int `yaml:"replicas,omitempty"`       // Service name -> replicas (only when > 1)
	Generated    string         `yaml:"generated_hash,omitempty"` // SHA256 hash of the generated content
}

// SecretsMount is the volume Portico adds to every service to expose app secrets
const SecretsMount = "./env:/run/secrets:ro"

// logsMountPrefix is the host prefix of the per-service logs mount added by the compose template
const logsMountPrefix = "/home/portico/logs/apps/"

// IsManagedVolume reports whether a volume entry is one Portico adds automatically
// (secrets mount or logs mount), so it must not be treated as a user volume.
func IsManagedVolume(vol string) bool {
	return vol == SecretsMount || strings.Contains(vol, ":/run/secrets") || strings.HasPrefix(vol, logsMountPrefix)
}

// ServiceAlias returns the DNS alias of a service on portico-network
func ServiceAlias(appName, serviceName string) string {
	return appName + "-" + serviceName
}

// TemplateFuncs are the helper functions available to docker-compose templates
var TemplateFuncs = template.FuncMap{
	// quote renders a string as a YAML double-quoted scalar (JSON strings are valid YAML)
	"quote": func(s string) string {
		b, err := json.Marshal(s)
		if err != nil {
			return `""`
		}
		return string(b)
	},
}

// templateMetadata returns the metadata to expose to the template (without the hash)
func templateMetadata(existing, metadata *PorticoMetadata) *PorticoMetadata {
	src := metadata
	if src == nil {
		src = existing
	}
	if src == nil {
		return nil
	}
	copied := *src
	copied.Generated = ""
	if copied.Port > 0 {
		copied.HttpEnabled = true
	}
	return &copied
}

// LoadComposeFile loads and parses an existing docker-compose.yml
func (dm *Manager) LoadComposeFile(appDir string) (*ComposeFile, error) {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	data, err := os.ReadFile(composeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ComposeFile{
				Services: make(map[string]interface{}),
				Networks: make(map[string]interface{}),
				Secrets:  make(map[string]interface{}),
			}, nil
		}
		return nil, fmt.Errorf("error reading docker-compose.yml: %w", err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("error parsing docker-compose.yml: %w", err)
	}

	if compose.Services == nil {
		compose.Services = make(map[string]interface{})
	}
	if compose.Networks == nil {
		compose.Networks = make(map[string]interface{})
	}
	if compose.Secrets == nil {
		compose.Secrets = make(map[string]interface{})
	}

	return &compose, nil
}

// TemplateService represents a service for the template
type TemplateService struct {
	Name        string
	Image       string
	Ports       []string
	Environment map[string]string
	Volumes     []string
	Secrets     []string
	DependsOn   []string
}

// TemplateSecret represents a secret for the template
type TemplateSecret struct {
	File string
}

// TemplateData represents data for docker-compose template
type TemplateData struct {
	AppName  string
	Services []TemplateService
	Secrets  map[string]TemplateSecret
	XPortico *PorticoMetadata
}

// GenerateDockerCompose generates/updates docker-compose.yml with intelligent merge using template
func (dm *Manager) GenerateDockerCompose(appDir string, services []Service, metadata *PorticoMetadata) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Load existing compose file to preserve custom fields
	existing, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return err
	}

	// Prepare template services.
	// The services passed in are the complete desired state (callers load the app
	// from docker-compose.yml, modify it and pass it back), so Portico-managed
	// fields (environment, volumes, ...) are taken from them as-is. Merging the old
	// values back in would make deletions (env/storage delete) impossible.
	templateServices := []TemplateService{}
	for _, svc := range services {
		env := make(map[string]string, len(svc.Environment))
		for k, v := range svc.Environment {
			env[k] = v
		}

		templateSvc := TemplateService{
			Name:        svc.Name,
			Image:       svc.Image,
			Environment: env,
			Secrets:     svc.Secrets,
			DependsOn:   svc.DependsOn,
		}

		// Handle ports - only expose ports explicitly added via ExtraPorts
		// Services communicate via internal DNS (service name) on portico-network
		// No need to expose ports to host for web services (Caddy uses internal DNS)
		templateSvc.Ports = append([]string{}, svc.ExtraPorts...)

		// User volumes (deduplicated, without the mounts Portico adds itself),
		// followed by the secrets mount.
		seen := make(map[string]bool)
		for _, vol := range svc.Volumes {
			if vol == "" || IsManagedVolume(vol) || seen[vol] {
				continue
			}
			seen[vol] = true
			templateSvc.Volumes = append(templateSvc.Volumes, vol)
		}
		templateSvc.Volumes = append(templateSvc.Volumes, SecretsMount)

		templateServices = append(templateServices, templateSvc)
	}

	// Prepare secrets for template
	templateSecrets := make(map[string]TemplateSecret)
	for _, svc := range services {
		for _, secret := range svc.Secrets {
			if _, exists := templateSecrets[secret]; !exists {
				templateSecrets[secret] = TemplateSecret{
					File: fmt.Sprintf("./env/%s", secret),
				}
			}
		}
	}

	// Extract app name from directory for project name
	appName := filepath.Base(appDir)

	// Prepare template data
	templateData := TemplateData{
		AppName:  appName,
		Services: templateServices,
		Secrets:  templateSecrets,
		XPortico: templateMetadata(existing.XPortico, metadata),
	}

	// Load template from filesystem first, then embedded files
	// We need templatesDir - we can get it from config or use default
	templatesDir := "/home/portico/templates" // Default, could be configurable
	if cfg, err := config.LoadConfig(); err == nil {
		templatesDir = cfg.TemplatesDir
	}
	templateDataBytes, err := embed.LoadTemplate(templatesDir, "docker-compose.tmpl")
	if err != nil {
		return fmt.Errorf("error reading docker-compose template: %w", err)
	}

	t, err := template.New("docker-compose").Funcs(TemplateFuncs).Parse(string(templateDataBytes))
	if err != nil {
		return fmt.Errorf("error parsing docker-compose template: %w", err)
	}

	var bufWithoutHash bytes.Buffer
	if err := t.Execute(&bufWithoutHash, templateData); err != nil {
		return fmt.Errorf("error executing docker-compose template: %w", err)
	}

	// Parse generated YAML to merge with existing custom fields
	var generated ComposeFile
	if err := yaml.Unmarshal(bufWithoutHash.Bytes(), &generated); err != nil {
		return fmt.Errorf("error parsing generated docker-compose: %w", err)
	}

	// Merge custom fields from existing compose (fields not managed by Portico)
	for svcName, existingSvc := range existing.Services {
		if existingSvcMap, ok := existingSvc.(map[string]interface{}); ok {
			// Check if this service is in generated services
			if generatedSvc, exists := generated.Services[svcName]; exists {
				if generatedSvcMap, ok := generatedSvc.(map[string]interface{}); ok {
					// Preserve custom fields that are not Portico-managed
					porticoManagedFields := map[string]bool{
						"image":       true,
						"ports":       true,
						"environment": true,
						"volumes":     true,
						"secrets":     true,
						"depends_on":  true,
						"logging":     true,
						"networks":    true,
					}
					for k, v := range existingSvcMap {
						if !porticoManagedFields[k] {
							generatedSvcMap[k] = v
						}
					}
					generated.Services[svcName] = generatedSvcMap
				}
			}
		}
	}

	// Preserve other top-level keys (named volumes, configs, x-* extensions, ...)
	for key, value := range existing.Extra {
		if _, managed := generated.Extra[key]; managed {
			continue
		}
		if generated.Extra == nil {
			generated.Extra = make(map[string]interface{})
		}
		generated.Extra[key] = value
	}

	// Preserve custom networks (if any other than portico-network)
	for netName, netConfig := range existing.Networks {
		if netName != "portico-network" {
			if generated.Networks == nil {
				generated.Networks = make(map[string]interface{})
			}
			generated.Networks[netName] = netConfig
		}
	}

	// Attach every service to portico-network with a unique "<app>-<service>" alias.
	// Docker Compose only registers "<service>" and "<app>-<service>-<n>" as DNS
	// names; "<service>" collides between apps sharing the network, so Caddy
	// proxies to the alias instead (it also resolves to every replica).
	for svcName, svc := range generated.Services {
		if svcMap, ok := svc.(map[string]interface{}); ok {
			svcMap["networks"] = map[string]interface{}{
				"portico-network": map[string]interface{}{
					"aliases": []string{ServiceAlias(appName, svcName)},
				},
			}
		}
	}

	// Set metadata
	previous := existing.XPortico
	if generated.XPortico == nil {
		generated.XPortico = &PorticoMetadata{}
	}
	if metadata != nil {
		generated.XPortico.Domain = metadata.Domain
		generated.XPortico.Port = metadata.Port
		// Set HttpEnabled: explicit value or infer from Port > 0
		if metadata.Port > 0 {
			generated.XPortico.HttpEnabled = true
		} else {
			generated.XPortico.HttpEnabled = metadata.HttpEnabled
		}
		generated.XPortico.HttpService = metadata.HttpService
	}
	// Remember the HTTP port while HTTP is disabled so it can be restored later
	if generated.XPortico.HttpEnabled {
		generated.XPortico.DisabledPort = 0
	} else {
		generated.XPortico.Port = 0
		switch {
		case metadata != nil && metadata.DisabledPort > 0:
			generated.XPortico.DisabledPort = metadata.DisabledPort
		case previous != nil && previous.HttpEnabled && previous.Port > 0:
			generated.XPortico.DisabledPort = previous.Port
		case previous != nil:
			generated.XPortico.DisabledPort = previous.DisabledPort
		}
	}
	// Keep the previously selected HTTP service when the caller didn't choose one
	if generated.XPortico.HttpService == "" && previous != nil {
		generated.XPortico.HttpService = previous.HttpService
	}
	// Forget the HTTP service if it no longer exists
	if _, ok := generated.Services[generated.XPortico.HttpService]; !ok {
		generated.XPortico.HttpService = ""
	}

	// Persist replica counts so up/reset/deploy keep the scale
	generated.XPortico.Replicas = nil
	for _, svc := range services {
		if svc.Replicas > 1 {
			if generated.XPortico.Replicas == nil {
				generated.XPortico.Replicas = make(map[string]int)
			}
			generated.XPortico.Replicas[svc.Name] = svc.Replicas
		}
	}

	// Calculate the hash of the content without the hash field itself
	hashStr, err := composeHash(&generated)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose.yml for hash: %w", err)
	}
	generated.XPortico.Generated = hashStr

	// Marshal final version with hash
	finalData, err := yaml.Marshal(generated)
	if err != nil {
		return fmt.Errorf("error marshaling final docker-compose: %w", err)
	}

	if err := util.WriteFileAtomic(composeFile, finalData, 0o644); err != nil {
		return fmt.Errorf("error writing docker-compose.yml: %w", err)
	}

	// Fix file ownership if running as root
	if err := util.FixFileOwnership(composeFile); err != nil {
		// Log warning but don't fail - ownership fix is best effort
		_ = err
	}

	return nil
}

// GetPorticoMetadata extracts Portico metadata from docker-compose.yml
func (dm *Manager) GetPorticoMetadata(appDir string) (*PorticoMetadata, error) {
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return nil, err
	}

	if compose.XPortico != nil {
		return compose.XPortico, nil
	}

	// Return defaults if not found
	return &PorticoMetadata{
		Domain:      "",
		Port:        0,
		HttpEnabled: false,
	}, nil
}

// DetectManualChanges checks if docker-compose.yml was manually modified
// by comparing its current hash with the stored hash in metadata
func (dm *Manager) DetectManualChanges(appDir string) (bool, error) {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	if _, err := os.Stat(composeFile); err != nil {
		if os.IsNotExist(err) {
			return false, nil // No compose file = not manually modified
		}
		return false, err
	}

	// Parse to get stored hash
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return false, err
	}

	// If no metadata or no hash, likely manually created/modified
	if compose.XPortico == nil || compose.XPortico.Generated == "" {
		return true, nil
	}

	storedHash := compose.XPortico.Generated

	currentHashStr, err := composeHash(compose)
	if err != nil {
		return false, err
	}

	// If stored hash doesn't match current hash, file was manually modified
	return currentHashStr != storedHash, nil
}

// Service represents a Docker service
type Service struct {
	Name        string
	Image       string
	Port        int
	ExtraPorts  []string
	Environment map[string]string
	Volumes     []string
	Secrets     []string
	DependsOn   []string
	Replicas    int // Number of instances (default: 1, 0 means 1)
}

// ensureNetworkExists ensures that a Docker network exists, creating it if necessary
func (dm *Manager) ensureNetworkExists(networkName string) error {
	// Check if network exists
	cmd := exec.Command("docker", "network", "inspect", networkName)
	if err := cmd.Run(); err == nil {
		// Network exists
		return nil
	}

	// Network doesn't exist, create it
	createCmd := exec.Command("docker", "network", "create", networkName)
	output, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating network %s: %s\n%s", networkName, err, string(output))
	}

	return nil
}

// SetServiceField sets a field that Portico does not manage (e.g. "command") on a
// service in docker-compose.yml. Unmanaged fields are preserved by
// GenerateDockerCompose, so callers should regenerate afterwards to refresh the hash.
func (dm *Manager) SetServiceField(appDir, serviceName, key string, value interface{}) error {
	compose, err := dm.LoadComposeFile(appDir)
	if err != nil {
		return err
	}
	svc, ok := compose.Services[serviceName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("service %s not found in docker-compose.yml", serviceName)
	}
	svc[key] = value

	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose.yml: %w", err)
	}
	return util.WriteFileAtomic(filepath.Join(appDir, "docker-compose.yml"), data, 0o644)
}

// legacyAddonProject reports whether an addon instance created before addon
// projects were namespaced uses name as its compose project
func legacyAddonProject(name string) bool {
	cfg, err := config.LoadConfig()
	if err != nil {
		return true // unknown: be conservative
	}
	data, err := os.ReadFile(filepath.Join(cfg.AddonsDir, "instances", name, "docker-compose.yml"))
	if err != nil {
		return false
	}
	var compose struct {
		Name string `yaml:"name"`
	}
	return yaml.Unmarshal(data, &compose) != nil || compose.Name == "" || compose.Name == name
}
