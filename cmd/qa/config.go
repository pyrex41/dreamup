package main

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config holds application configuration
type Config struct {
	DefaultOutputDir string
	DefaultHeadless  bool
	DefaultMaxDuration int
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.dreamup")

	// Set defaults
	viper.SetDefault("output_dir", "./qa-results")
	viper.SetDefault("headless", true)
	viper.SetDefault("max_duration", 300)

	// Read environment variables
	viper.SetEnvPrefix("DREAMUP")
	viper.AutomaticEnv()

	// Read config file (optional - don't fail if missing)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found is OK - we'll use defaults
	}

	config := &Config{
		DefaultOutputDir:   viper.GetString("output_dir"),
		DefaultHeadless:    viper.GetBool("headless"),
		DefaultMaxDuration: viper.GetInt("max_duration"),
	}

	return config, nil
}

// EnsureOutputDir creates the output directory if it doesn't exist
func EnsureOutputDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}
	return nil
}
