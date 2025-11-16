package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	envAPIKey = "CHATTY_API_KEY"
	envAPIURL = "CHATTY_API_URL"
)

// Config captures runtime configuration for the Chatty application.
type Config struct {
	API     APIConfig     `yaml:"api"`
	Model   ModelConfig   `yaml:"model"`
	Logging LoggingConfig `yaml:"logging"`
	UI      UIConfig      `yaml:"ui"`
	Storage StorageConfig `yaml:"storage"`
}

// APIConfig holds settings for connecting to the OpenAI-compatible API.
type APIConfig struct {
	URL string `yaml:"url"`
	Key string `yaml:"key"`
}

// ModelConfig controls default model behaviour.
type ModelConfig struct {
	Name        string  `yaml:"name"`
	Temperature float64 `yaml:"temperature"`
	Stream      bool    `yaml:"stream"`
}

// LoggingConfig encapsulates logging preferences.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// UIConfig defines terminal rendering preferences.
type UIConfig struct {
	ShowTimestamps bool `yaml:"show_timestamps"`
}

// StorageConfig defines persistence options.
type StorageConfig struct {
	Path string `yaml:"path"`
}

// Load reads configuration from the provided path, falling back to defaults and
// environment overrides.
func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	if path != "" {
		if err := loadFile(path, &cfg); err != nil {
			return nil, err
		}
	} else {
		if err := loadFile("config.yaml", &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	applyEnvOverrides(&cfg)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return err
		}
		return fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Expand environment variables in config values
	cfg.API.Key = os.ExpandEnv(cfg.API.Key)
	cfg.API.URL = os.ExpandEnv(cfg.API.URL)
	cfg.Storage.Path = os.ExpandEnv(cfg.Storage.Path)

	return nil
}

func applyEnvOverrides(cfg *Config) {
	if url := strings.TrimSpace(os.Getenv(envAPIURL)); url != "" {
		cfg.API.URL = url
	}
	if key := strings.TrimSpace(os.Getenv(envAPIKey)); key != "" {
		cfg.API.Key = key
	}
}

func (c *Config) validate() error {
	var validationErrors []string

	// API URL validation
	if strings.TrimSpace(c.API.URL) == "" {
		validationErrors = append(validationErrors, "API URL (api.url) must be configured")
	} else {
		if !strings.HasPrefix(c.API.URL, "http://") && !strings.HasPrefix(c.API.URL, "https://") {
			validationErrors = append(validationErrors, "API URL must start with http:// or https://")
		} else {
			if _, parseErr := url.Parse(c.API.URL); parseErr != nil {
				validationErrors = append(validationErrors, fmt.Sprintf("API URL is invalid: %v", parseErr))
			}
		}
	}

	// API Key validation
	if strings.Contains(c.API.Key, "${") {
		validationErrors = append(validationErrors, "API key contains unexpanded environment variable, set CHATTY_API_KEY environment variable or replace ${...} in config")
	}
	if strings.TrimSpace(c.API.Key) == "" {
		validationErrors = append(validationErrors, "API key (api.key) must be set or CHATTY_API_KEY environment variable must be provided")
	}

	// Model validation
	if strings.TrimSpace(c.Model.Name) == "" {
		validationErrors = append(validationErrors, "Model name (model.name) cannot be empty")
	} else if len(c.Model.Name) > 200 {
		validationErrors = append(validationErrors, "Model name (model.name) exceeds maximum length of 200 characters")
	}

	// Temperature validation
	if c.Model.Temperature < 0.0 || c.Model.Temperature > 2.0 {
		validationErrors = append(validationErrors, fmt.Sprintf("Model temperature (model.temperature) must be between 0.0 and 2.0, got %.2f", c.Model.Temperature))
	}

	// Logging level validation
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if strings.TrimSpace(c.Logging.Level) == "" {
		validationErrors = append(validationErrors, "Logging level (logging.level) cannot be empty")
	} else {
		isValidLevel := false
		for _, validLevel := range validLevels {
			if strings.EqualFold(c.Logging.Level, validLevel) {
				isValidLevel = true
				break
			}
		}
		if !isValidLevel {
			validationErrors = append(validationErrors, fmt.Sprintf("Logging level (logging.level) must be one of: %v, got %s", validLevels, c.Logging.Level))
		}
	}

	// Storage path validation
	if strings.TrimSpace(c.Storage.Path) != "" {
		if info, statErr := os.Stat(c.Storage.Path); statErr == nil {
			if !info.IsDir() {
				validationErrors = append(validationErrors, fmt.Sprintf("Storage path (%s) must be a directory, not a file", c.Storage.Path))
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("configuration validation failed:\n\t• %s", strings.Join(validationErrors, "\n\t• "))
	}

	return nil
}

func defaultConfig() Config {
	return Config{
		API: APIConfig{
			URL: "",
		},
		Model: ModelConfig{
			Name:        "groq/moonshotai/kimi-k2-instruct-0905",
			Temperature: 0.7,
			Stream:      true,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		UI: UIConfig{
			ShowTimestamps: true,
		},
		Storage: StorageConfig{
			Path: "",
		},
	}
}
