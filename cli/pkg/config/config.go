package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the ClusterKit configuration
type Config struct {
	// GCP Settings
	ProjectID   string `mapstructure:"project_id"`
	Region      string `mapstructure:"region"`
	ClusterName string `mapstructure:"cluster_name"`

	// Domain Settings
	Domain string `mapstructure:"domain"`

	// Cloudflare Settings
	CloudflareToken string `mapstructure:"cloudflare_token"`

	// Kubernetes Settings
	Kubeconfig string `mapstructure:"kubeconfig"`
	Context    string `mapstructure:"context"`

	// Default App Settings
	Defaults AppDefaults `mapstructure:"defaults"`

	// Logging Settings
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

// AppDefaults contains default values for new applications
type AppDefaults struct {
	MinScale    int    `mapstructure:"min_scale"`
	MaxScale    int    `mapstructure:"max_scale"`
	Concurrency int    `mapstructure:"concurrency"`
	Memory      string `mapstructure:"memory"`
	CPU         string `mapstructure:"cpu"`
}

// Load loads configuration from file and environment variables
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file if specified
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in default locations
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}

		// Add config search paths
		v.AddConfigPath(filepath.Join(home, ".clusterkit"))
		v.AddConfigPath(".")
		v.SetConfigType("yaml")
		v.SetConfigName("config")
	}

	// Environment variables
	v.SetEnvPrefix("CLUSTERKIT")
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// Config file not required, can use env vars and defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// GCP defaults
	v.SetDefault("region", "us-central1")
	v.SetDefault("cluster_name", "clusterkit")

	// Kubernetes defaults
	v.SetDefault("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"))

	// App defaults
	v.SetDefault("defaults.min_scale", 0)
	v.SetDefault("defaults.max_scale", 10)
	v.SetDefault("defaults.concurrency", 10)
	v.SetDefault("defaults.memory", "256Mi")
	v.SetDefault("defaults.cpu", "1000m")

	// Logging defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate log format
	validLogFormats := map[string]bool{
		"text": true,
		"json": true,
	}

	if !validLogFormats[c.LogFormat] {
		return fmt.Errorf("invalid log_format: %s (must be text or json)", c.LogFormat)
	}

	// Validate app defaults
	if c.Defaults.MinScale < 0 {
		return fmt.Errorf("defaults.min_scale must be >= 0")
	}

	if c.Defaults.MaxScale <= 0 {
		return fmt.Errorf("defaults.max_scale must be > 0")
	}

	if c.Defaults.MinScale > c.Defaults.MaxScale {
		return fmt.Errorf("defaults.min_scale must be <= defaults.max_scale")
	}

	if c.Defaults.Concurrency <= 0 {
		return fmt.Errorf("defaults.concurrency must be > 0")
	}

	return nil
}

// Save saves the configuration to a file
func (c *Config) Save(path string) error {
	v := viper.New()
	v.SetConfigFile(path)

	// Set all config values
	v.Set("project_id", c.ProjectID)
	v.Set("region", c.Region)
	v.Set("cluster_name", c.ClusterName)
	v.Set("domain", c.Domain)
	v.Set("cloudflare_token", c.CloudflareToken)
	v.Set("kubeconfig", c.Kubeconfig)
	v.Set("context", c.Context)
	v.Set("defaults", c.Defaults)
	v.Set("log_level", c.LogLevel)
	v.Set("log_format", c.LogFormat)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".clusterkit/config.yaml"
	}
	return filepath.Join(home, ".clusterkit", "config.yaml")
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(path string) error {
	cfg := &Config{
		Region:      "us-central1",
		ClusterName: "clusterkit",
		Defaults: AppDefaults{
			MinScale:    0,
			MaxScale:    10,
			Concurrency: 10,
			Memory:      "256Mi",
			CPU:         "1000m",
		},
		LogLevel:  "info",
		LogFormat: "text",
	}

	return cfg.Save(path)
}
