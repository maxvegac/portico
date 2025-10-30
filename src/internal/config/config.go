package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the Portico configuration
type Config struct {
	PorticoHome  string         `yaml:"portico_home"`
	AppsDir      string         `yaml:"apps_dir"`
	ProxyDir     string         `yaml:"proxy_dir"`
	TemplatesDir string         `yaml:"templates_dir"`
	Registry     RegistryConfig `yaml:"registry"`
}

// RegistryConfig represents Docker registry configuration
type RegistryConfig struct {
	Type     string `yaml:"type"` // "internal" or "external"
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// isRunningAsRoot checks if the current process is running as root
func isRunningAsRoot() bool {
	return os.Geteuid() == 0
}

// isUserInPorticoGroup removed: avoid os/user dependency; rely on path checks instead

// canAccessPorticoHome checks if we can access /home/portico
func canAccessPorticoHome() bool {
	if _, err := os.Stat("/home/portico"); err != nil {
		return false
	}
	return true
}

// getConfigPaths returns appropriate config paths based on execution context
func getConfigPaths() []string {
	paths := []string{
		".",        // Current directory
		"./static", // Static config in project
	}

	// Add system paths based on access level
	if isRunningAsRoot() {
		paths = append(paths, "/etc/portico", "/home/portico") // System-wide config and Portico home
	} else if canAccessPorticoHome() {
		paths = append(paths, "/home/portico") // Portico home
	}

	return paths
}

// LoadConfig loads the Portico configuration
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config paths based on execution context
	for _, path := range getConfigPaths() {
		viper.AddConfigPath(path)
	}

	// Set default values
	viper.SetDefault("portico_home", "/home/portico")
	viper.SetDefault("apps_dir", "/home/portico/apps")
	viper.SetDefault("proxy_dir", "/home/portico/reverse-proxy")
	viper.SetDefault("templates_dir", "/home/portico/templates")
	viper.SetDefault("registry.type", "internal")
	viper.SetDefault("registry.url", "localhost:5000")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, use defaults
	}

	// Create config manually from viper values
	config := &Config{
		PorticoHome:  viper.GetString("portico_home"),
		AppsDir:      viper.GetString("apps_dir"),
		ProxyDir:     viper.GetString("proxy_dir"),
		TemplatesDir: viper.GetString("templates_dir"),
		Registry: RegistryConfig{
			Type:     viper.GetString("registry.type"),
			URL:      viper.GetString("registry.url"),
			Username: viper.GetString("registry.username"),
			Password: viper.GetString("registry.password"),
		},
	}

	return config, nil
}

// SaveConfig saves the configuration to file
func (c *Config) SaveConfig() error {
	configPath := filepath.Join(c.PorticoHome, "config.yml")

	// Ensure directory exists
	if err := os.MkdirAll(c.PorticoHome, 0o755); err != nil {
		return fmt.Errorf("error creating portico home directory: %w", err)
	}

	// Set viper values
	viper.Set("portico_home", c.PorticoHome)
	viper.Set("apps_dir", c.AppsDir)
	viper.Set("proxy_dir", c.ProxyDir)
	viper.Set("templates_dir", c.TemplatesDir)
	viper.Set("registry", c.Registry)

	return viper.WriteConfigAs(configPath)
}
