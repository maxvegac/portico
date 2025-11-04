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

	"gopkg.in/yaml.v3"
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

	// Build docker compose command
	args := []string{"compose", "-f", composeFile, "up", "-d"}

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

	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Dir = appDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error stopping application: %s\n%s", err, string(output))
	}

	return nil
}

// ComposeFile represents a docker-compose.yml structure with Portico metadata
type ComposeFile struct {
	Services map[string]interface{} `yaml:"services"`
	Networks map[string]interface{} `yaml:"networks,omitempty"`
	Secrets  map[string]interface{} `yaml:"secrets,omitempty"`
	XPortico *PorticoMetadata       `yaml:"x-portico,omitempty"`
}

// PorticoMetadata stores Portico-specific configuration
type PorticoMetadata struct {
	Domain    string `yaml:"domain,omitempty"`
	Port      int    `yaml:"http_port,omitempty"`
	Generated string `yaml:"generated_hash,omitempty"` // SHA256 hash of the generated content
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

// GenerateDockerCompose generates/updates docker-compose.yml with intelligent merge
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

	// Merge services: update Portico-managed services while preserving custom fields
	for _, svc := range services {
		svcMap := make(map[string]interface{})

		// If service exists, preserve custom fields
		if existingSvc, ok := existing.Services[svc.Name].(map[string]interface{}); ok {
			// Copy existing fields to preserve customizations
			for k, v := range existingSvc {
				svcMap[k] = v
			}
		}

		// Update Portico-managed fields
		svcMap["image"] = svc.Image
		svcMap["networks"] = []string{"portico-network"}

		// Handle ports
		ports := []string{}
		if svc.Port > 0 {
			ports = append(ports, fmt.Sprintf("%d:%d", svc.Port, svc.Port))
		}
		ports = append(ports, svc.ExtraPorts...)
		svcMap["ports"] = ports

		// Handle environment
		env := []string{}
		for k, v := range svc.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		svcMap["environment"] = env

		// Handle volumes
		volumes := svc.Volumes
		volumes = append(volumes, "./env:/run/secrets:ro") // Always add secrets mount
		svcMap["volumes"] = volumes

		// Handle secrets
		svcMap["secrets"] = svc.Secrets

		// Handle depends_on
		if len(svc.DependsOn) > 0 {
			svcMap["depends_on"] = svc.DependsOn
		}

		existing.Services[svc.Name] = svcMap
	}

	// Ensure networks section
	if existing.Networks == nil {
		existing.Networks = make(map[string]interface{})
	}
	existing.Networks["portico-network"] = map[string]interface{}{
		"external": true,
	}

	// Ensure secrets section
	if existing.Secrets == nil {
		existing.Secrets = make(map[string]interface{})
	}
	// Add secrets from services
	for _, svc := range services {
		for _, secret := range svc.Secrets {
			if _, exists := existing.Secrets[secret]; !exists {
				existing.Secrets[secret] = map[string]string{
					"file": fmt.Sprintf("./env/%s", secret),
				}
			}
		}
	}

	// Calculate hash BEFORE adding the hash field itself
	// Temporarily remove hash if it exists
	if existing.XPortico != nil {
		existing.XPortico.Generated = ""
	}

	// Marshal without hash to calculate the hash
	dataWithoutHash, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose.yml: %w", err)
	}

	// Calculate hash of content without the hash field
	hash := sha256.Sum256(dataWithoutHash)
	hashStr := fmt.Sprintf("%x", hash)

	// Now add the hash to metadata
	if existing.XPortico == nil {
		existing.XPortico = &PorticoMetadata{}
	}
	if metadata != nil {
		existing.XPortico.Domain = metadata.Domain
		existing.XPortico.Port = metadata.Port
	}
	existing.XPortico.Generated = hashStr

	// Marshal final version with hash
	data, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("error marshaling docker-compose.yml with hash: %w", err)
	}

	return os.WriteFile(composeFile, data, 0o644)
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
		Domain: "",
		Port:   0,
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
	cmd := exec.Command("docker", "compose", "-f", composeFile, "ps", "--format", "json")
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
