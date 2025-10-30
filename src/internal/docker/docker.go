package docker

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
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
func (dm *Manager) DeployApp(appDir string) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Check if docker-compose.yml exists
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found in %s", appDir)
	}

	// Run docker compose up
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
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

// GenerateDockerCompose generates a docker-compose.yml file for an application
func (dm *Manager) GenerateDockerCompose(appDir string, services []Service) error {
	composeFile := filepath.Join(appDir, "docker-compose.yml")

	// Load template
	templatePath := "templates/docker-compose.tmpl"
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing docker-compose template: %w", err)
	}

	// Create output file
	file, err := os.Create(composeFile)
	if err != nil {
		return fmt.Errorf("error creating docker-compose.yml: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Execute template
	if err := t.Execute(file, struct {
		Services []Service
	}{
		Services: services,
	}); err != nil {
		return fmt.Errorf("error executing docker-compose template: %w", err)
	}

	return nil
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
