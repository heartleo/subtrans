// Package config handles loading and validating application configuration.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	APIKey      string  `mapstructure:"api_key"`
	BaseURL     string  `mapstructure:"base_url"`
	Model       string  `mapstructure:"model"`
	Temperature float64 `mapstructure:"temperature"`
	MaxRetries  int     `mapstructure:"max_retries"`
}

// Validate returns an error if required fields are missing.
func (c Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("api_key is required")
	}

	return nil
}

// Load reads configuration from .env file, environment variables and config files.
// Priority: env vars > .env file > config file > defaults.
func Load() (Config, error) {
	// Load .env file if present (does not override existing env vars)
	_ = godotenv.Load()

	v := viper.New()

	v.SetDefault("base_url", "https://api.openai.com/v1")
	v.SetDefault("model", "gpt-4.1")
	v.SetDefault("temperature", 0.0)
	v.SetDefault("max_retries", 3)

	v.SetConfigName("subtrans")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
	}

	_ = v.ReadInConfig() // not an error if config file doesn't exist

	v.AutomaticEnv()

	// Bind config keys to the OPENAI_* env vars from .env
	_ = v.BindEnv("api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("base_url", "OPENAI_BASE_URL")
	_ = v.BindEnv("model", "OPENAI_MODEL")
	_ = v.BindEnv("temperature", "OPENAI_TEMPERATURE")
	_ = v.BindEnv("max_retries", "OPENAI_MAX_RETRIES")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}
