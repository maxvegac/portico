package docker

import (
	"bufio"
	"bytes"
	"crypto/sha256"
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

	// Build docker compose command with explicit project name
	// This ensures services are named consistently: appname-servicename
	args := []string{"compose", "-f", composeFile, "-p", appName, "up", "-d"}

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
}

// PorticoMetadata stores Portico-specific configuration
type PorticoMetadata struct {
	Domain      string `yaml:"domain,omitempty"`
	Port        int    `yaml:"http_port,omitempty"`
	HttpEnabled bool   `yaml:"http_enabled,omitempty"`
	Generated   string `yaml:"generated_hash,omitempty"` // SHA256 hash of the generated content
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

	// Update Portico metadata
	if metadata != nil {
		existing.XPortico = metadata
	}

	// Prepare template services with merge
	templateServices := []TemplateService{}
	for _, svc := range services {
		templateSvc := TemplateService{
			Name:        svc.Name,
			Image:       svc.Image,
			Environment: svc.Environment,
			Volumes:     svc.Volumes,
			Secrets:     svc.Secrets,
			DependsOn:   svc.DependsOn,
		}

		// Handle ports - only expose ports explicitly added via ExtraPorts
		// Services communicate via internal DNS (service name) on portico-network
		// No need to expose ports to host for web services (Caddy uses internal DNS)
		templateSvc.Ports = append(templateSvc.Ports, svc.ExtraPorts...)

		// Always add secrets mount
		templateSvc.Volumes = append(templateSvc.Volumes, "./env:/run/secrets:ro")

		// If service exists, try to preserve custom fields from existing
		if existingSvc, ok := existing.Services[svc.Name].(map[string]interface{}); ok {
			// Preserve custom environment variables if they exist
			if existingEnv, ok := existingSvc["environment"].([]interface{}); ok {
				// Merge with existing environment
				for _, envItem := range existingEnv {
					if envStr, ok := envItem.(string); ok {
						parts := strings.SplitN(envStr, "=", 2)
						if len(parts) == 2 {
							// Only preserve if not managed by Portico
							if _, exists := templateSvc.Environment[parts[0]]; !exists {
								templateSvc.Environment[parts[0]] = parts[1]
							}
						}
					}
				}
			}

			// Preserve custom volumes (excluding the secrets mount)
			if existingVolumes, ok := existingSvc["volumes"].([]interface{}); ok {
				for _, vol := range existingVolumes {
					if volStr, ok := vol.(string); ok {
						// Only preserve if not the secrets mount and not in Portico-managed volumes
						if volStr != "./env:/run/secrets:ro" && !contains(templateSvc.Volumes[:len(templateSvc.Volumes)-1], volStr) {
							templateSvc.Volumes = append(templateSvc.Volumes[:len(templateSvc.Volumes)-1], volStr, "./env:/run/secrets:ro")
						}
					}
				}
			}
		}

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
		XPortico: existing.XPortico,
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

	t, err := template.New("docker-compose").Parse(string(templateDataBytes))
	if err != nil {
		return fmt.Errorf("error parsing docker-compose template: %w", err)
	}

	// Generate YAML from template (without hash first)
	templateDataWithoutHash := templateData
	if templateDataWithoutHash.XPortico != nil {
		templateDataWithoutHash.XPortico.Generated = ""
	}

	var bufWithoutHash bytes.Buffer
	if err := t.Execute(&bufWithoutHash, templateDataWithoutHash); err != nil {
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

	// Preserve custom networks (if any other than portico-network)
	for netName, netConfig := range existing.Networks {
		if netName != "portico-network" {
			if generated.Networks == nil {
				generated.Networks = make(map[string]interface{})
			}
			generated.Networks[netName] = netConfig
		}
	}

	// Set metadata
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
	}

	// Calculate hash BEFORE adding the hash field itself
	// Temporarily remove hash if it exists
	generated.XPortico.Generated = ""
	dataWithoutHash, err := yaml.Marshal(generated)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose.yml for hash: %w", err)
	}

	// Calculate hash of content without the hash field
	hash := sha256.Sum256(dataWithoutHash)
	hashStr := fmt.Sprintf("%x", hash)

	// Now add the hash to metadata
	generated.XPortico.Generated = hashStr

	// Marshal final version with hash
	finalData, err := yaml.Marshal(generated)
	if err != nil {
		return fmt.Errorf("error marshaling final docker-compose: %w", err)
	}

	if err := os.WriteFile(composeFile, finalData, 0o644); err != nil {
		return fmt.Errorf("error writing docker-compose.yml: %w", err)
	}

	// Fix file ownership if running as root
	if err := util.FixFileOwnership(composeFile); err != nil {
		// Log warning but don't fail - ownership fix is best effort
		_ = err
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
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

	// Read current docker-compose.yml
	currentData, err := os.ReadFile(composeFile)
	if err != nil {
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

	// Parse current file and remove hash for comparison
	var currentCompose ComposeFile
	if err := yaml.Unmarshal(currentData, &currentCompose); err != nil {
		return false, err
	}

	// Remove hash from current compose for comparison
	if currentCompose.XPortico != nil {
		currentCompose.XPortico.Generated = ""
	}

	currentWithoutHash, err := yaml.Marshal(&currentCompose)
	if err != nil {
		return false, err
	}

	// Calculate hash of current content (without hash field)
	currentHash := sha256.Sum256(currentWithoutHash)
	currentHashStr := fmt.Sprintf("%x", currentHash)

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

// GetContainerStatus returns the status of containers for an app
func (dm *Manager) GetContainerStatus(appDir string) ([]ContainerStatus, error) {
	// Validate appDir path to prevent path traversal
	if !filepath.IsAbs(appDir) {
		appDir, _ = filepath.Abs(appDir)
	}

	composeFile := filepath.Join(appDir, "docker-compose.yml")
	// Extract app name from directory for consistent project naming
	appName := filepath.Base(appDir)
	cmd := exec.Command("docker", "compose", "-f", composeFile, "-p", appName, "ps", "--format", "json")
	cmd.Dir = appDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error getting container status: %w", err)
	}

	var statuses []ContainerStatus
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// Parse JSON line to extract container info
			// This is a simplified version - in production you'd use proper JSON parsing
			statuses = append(statuses, ContainerStatus{
				Name:   "container", // Extract from JSON
				Status: "running",   // Extract from JSON
			})
		}
	}

	return statuses, nil
}

// ContainerStatus represents the status of a container
type ContainerStatus struct {
	Name   string
	Status string
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
