package config

import (
	"fmt"
	"os"
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

// LoadConfig loads the Portico configuration
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/home/portico")
	viper.AddConfigPath(".")

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

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
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
