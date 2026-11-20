package addon

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/embed"
	"github.com/maxvegac/portico/src/internal/util"
)

// Manager handles addon operations
type Manager struct {
	AddonsDir    string
	ConfigFile   string
	InstancesDir string
}

// NewManager creates a new addon manager
func NewManager(addonsDir, instancesDir string) *Manager {
	return &Manager{
		AddonsDir:    addonsDir,
		ConfigFile:   filepath.Join(addonsDir, "config.yml"),
		InstancesDir: instancesDir,
	}
}

// Definition represents an addon definition YAML
type Definition struct {
	Name        string                   `yaml:"name"`
	Type        string                   `yaml:"type"` // "database", "cache", "tool"
	Description string                   `yaml:"description"`
	Versions    map[string]VersionConfig `yaml:"versions"` // Version -> config
	DefaultPort int                      `yaml:"default_port"`
	ServiceMode string                   `yaml:"service_mode"` // "shared", "dedicated", "inline"
}

// VersionConfig represents configuration for a specific version
type VersionConfig struct {
	Image       string            `yaml:"image"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []VolumeConfig    `yaml:"volumes"`
	Secrets     []string          `yaml:"secrets"`
	Ports       []PortConfig      `yaml:"ports"`
}

// VolumeConfig represents volume configuration
type VolumeConfig struct {
	HostPath      string `yaml:"host_path"`
	ContainerPath string `yaml:"container_path"`
	Type          string `yaml:"type"` // "bind", "volume"
}

// PortConfig represents port configuration
type PortConfig struct {
	Internal int `yaml:"internal"`
	External int `yaml:"external,omitempty"`
}

// LoadDefinition loads an addon definition from a YAML file
func (am *Manager) LoadDefinition(addonType string) (*Definition, error) {
	// Try to load from installed addons first
	defPath := filepath.Join(am.AddonsDir, "definitions", fmt.Sprintf("%s.yml", addonType))

	var data []byte

	// Try to read from filesystem first
	if _, err := os.Stat(defPath); err == nil {
		var readErr error
		data, readErr = os.ReadFile(defPath)
		if readErr != nil {
			return nil, fmt.Errorf("error reading addon definition: %w", readErr)
		}
	} else {
		// If not found in filesystem, try embedded files
		embedPath := fmt.Sprintf("static/addons/definitions/%s.yml", addonType)
		var readErr error
		data, readErr = embed.StaticFiles.ReadFile(embedPath)
		if readErr != nil {
			return nil, fmt.Errorf("error reading addon definition from embed: %w", readErr)
		}
	}

	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("error parsing addon definition: %w", err)
	}

	return &def, nil
}

// GetAvailableVersions returns the versions of an addon, newest first
func (def *Definition) GetAvailableVersions() []string {
	versions := make([]string, 0, len(def.Versions))
	for version := range def.Versions {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return compareVersions(versions[i], versions[j]) > 0 })
	return versions
}

// LatestVersion returns the newest available version ("" if none)
func (def *Definition) LatestVersion() string {
	if versions := def.GetAvailableVersions(); len(versions) > 0 {
		return versions[0]
	}
	return ""
}

// GetVersionConfig returns the configuration of a version (the latest one if version is empty)
func (def *Definition) GetVersionConfig(version string) (*VersionConfig, error) {
	if version == "" {
		version = def.LatestVersion()
		if version == "" {
			return nil, fmt.Errorf("no versions available")
		}
	}

	config, ok := def.Versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found. Available versions: %v", version, def.GetAvailableVersions())
	}

	return &config, nil
}

// compareVersions compares dotted version strings numerically ("8.4.7" > "8.4" > "5.7")
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na != nb {
			if na > nb {
				return 1
			}
			return -1
		}
	}
	return strings.Compare(a, b)
}

type Instance struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Version string   `yaml:"version"`        // Selected version
	Mode    string   `yaml:"mode"`           // "shared" or "dedicated"
	App     string   `yaml:"app,omitempty"`  // For dedicated mode
	Apps    []string `yaml:"apps,omitempty"` // For shared mode
	Port    int      `yaml:"port"`
	Domain  string   `yaml:"domain,omitempty"` // Public domain (admin tools published by Caddy)
	Target  string   `yaml:"target,omitempty"` // Database instance managed by an admin tool
	DataDir string   `yaml:"data_dir"`
}

// Config represents the addons configuration file
type Config struct {
	Instances map[string]Instance `yaml:"instances"`
}

// LoadConfig loads the addons configuration
func (am *Manager) LoadConfig() (*Config, error) {
	if _, err := os.Stat(am.ConfigFile); os.IsNotExist(err) {
		return &Config{
			Instances: make(map[string]Instance),
		}, nil
	}

	data, err := os.ReadFile(am.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading addons config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing addons config: %w", err)
	}

	if config.Instances == nil {
		config.Instances = make(map[string]Instance)
	}

	return &config, nil
}

// SaveConfig saves the addons configuration
func (am *Manager) SaveConfig(config *Config) error {
	// Ensure directory exists
	if err := os.MkdirAll(am.AddonsDir, 0o755); err != nil {
		return fmt.Errorf("error creating addons directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	return util.WriteFileAtomic(am.ConfigFile, data, 0o644)
}
