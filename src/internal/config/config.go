package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the Portico configuration
type Config struct {
	PorticoHome string         `yaml:"portico_home"`
	AppsDir     string         `yaml:"apps_dir"`
	ProxyDir    string         `yaml:"proxy_dir"`
	Registry    RegistryConfig `yaml:"registry"`
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

// isUserInPorticoGroup checks if the current user is in the portico group
func isUserInPorticoGroup() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	
	// Get user groups
	groups, err := currentUser.GroupIds()
	if err != nil {
		return false
	}
	
	// Look for portico group
	for _, gid := range groups {
		group, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		if group.Name == "portico" {
			return true
		}
	}
	
	return false
}

// canAccessPorticoHome checks if we can access /home/portico
func canAccessPorticoHome() bool {
	if _, err := os.Stat("/home/portico"); err != nil {
		return false
	}
	
	// Try to read the directory
	entries, err := os.ReadDir("/home/portico")
	if err != nil {
		return false
	}
	
	// If we can read entries, we have access
	return len(entries) >= 0
}

// getConfigPaths returns appropriate config paths based on execution context
func getConfigPaths() []string {
	paths := []string{
		".",              // Current directory
		"./static",       // Static config in project
	}
	
	// Add system paths based on access level
	if isRunningAsRoot() {
		paths = append(paths, "/etc/portico")   // System-wide config
		paths = append(paths, "/home/portico")  // Portico home
	} else if isUserInPorticoGroup() || canAccessPorticoHome() {
		paths = append(paths, "/home/portico")  // Portico home
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
		PorticoHome: viper.GetString("portico_home"),
		AppsDir:     viper.GetString("apps_dir"),
		ProxyDir:    viper.GetString("proxy_dir"),
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
	viper.Set("registry", c.Registry)

	return viper.WriteConfigAs(configPath)
}
