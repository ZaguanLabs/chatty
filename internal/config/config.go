package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	chattyErrors "github.com/ZaguanLabs/chatty/internal/errors"
)

const (
	envAPIKey = "CHATTY_API_KEY"
	envAPIURL = "CHATTY_API_URL"
	minAPIKeyLength = 16  // Increased minimum length for better security
	maxAPIKeyLength = 500 // Maximum length to prevent DoS
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
// environment overrides. This is the legacy function - use SecureLoad for better security.
func Load(path string) (*Config, error) {
	return SecureLoad(path)
}

// SecureLoad reads configuration from the provided path with enhanced security features
func SecureLoad(path string) (*Config, error) {
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
	var validationErrors []error

	// API URL validation
	if strings.TrimSpace(c.API.URL) == "" {
		validationErrors = append(validationErrors, chattyErrors.NewValidationError("api.url", "must be configured", c.API.URL, nil))
	} else {
		if !strings.HasPrefix(c.API.URL, "http://") && !strings.HasPrefix(c.API.URL, "https://") {
			validationErrors = append(validationErrors, chattyErrors.NewValidationError("api.url", "must start with http:// or https://", c.API.URL, nil))
		} else {
			if _, parseErr := url.Parse(c.API.URL); parseErr != nil {
				validationErrors = append(validationErrors, chattyErrors.NewValidationError("api.url", "is invalid", c.API.URL, parseErr))
			}
		}
	}

	// API Key validation with enhanced security checks
	if err := validateAPIKeySecure(c.API.Key); err != nil {
		validationErrors = append(validationErrors, chattyErrors.NewConfigError("api.key", err.Error(), nil))
	}

	// Model validation
	if strings.TrimSpace(c.Model.Name) == "" {
		validationErrors = append(validationErrors, chattyErrors.NewValidationError("model.name", "cannot be empty", c.Model.Name, nil))
	} else if len(c.Model.Name) > 200 {
		validationErrors = append(validationErrors, chattyErrors.NewValidationError("model.name", "exceeds maximum length of 200 characters", c.Model.Name, nil))
	}

	// Temperature validation
	if c.Model.Temperature < 0.0 || c.Model.Temperature > 2.0 {
		validationErrors = append(validationErrors, chattyErrors.NewValidationError("model.temperature", fmt.Sprintf("must be between 0.0 and 2.0, got %.2f", c.Model.Temperature), c.Model.Temperature, nil))
	}

	// Logging level validation
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if strings.TrimSpace(c.Logging.Level) == "" {
		validationErrors = append(validationErrors, chattyErrors.NewValidationError("logging.level", "cannot be empty", c.Logging.Level, nil))
	} else {
		isValidLevel := false
		for _, validLevel := range validLevels {
			if strings.EqualFold(c.Logging.Level, validLevel) {
				isValidLevel = true
				break
			}
		}
		if !isValidLevel {
			validationErrors = append(validationErrors, chattyErrors.NewValidationError("logging.level", fmt.Sprintf("must be one of: %v", validLevels), c.Logging.Level, nil))
		}
	}

	// Storage path validation
	if strings.TrimSpace(c.Storage.Path) != "" {
		if info, statErr := os.Stat(c.Storage.Path); statErr == nil {
			if !info.IsDir() {
				validationErrors = append(validationErrors, chattyErrors.NewValidationError("storage.path", fmt.Sprintf("must be a directory, not a file"), c.Storage.Path, nil))
			}
		}
	}

	if len(validationErrors) > 0 {
		return chattyErrors.NewConfigError("configuration", fmt.Sprintf("validation failed:\n\t• %s", strings.Join(getErrorMessages(validationErrors), "\n\t• ")), nil)
	}

	return nil
}

func getErrorMessages(errs []error) []string {
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return messages
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

// validateAPIKeySecure performs enhanced security validation on API keys
func validateAPIKeySecure(key string) error {
	key = strings.TrimSpace(key)

	if key == "" {
		return errors.New("API key cannot be empty")
	}

	// Enforce minimum key length
	if len(key) < minAPIKeyLength {
		return fmt.Errorf("API key too short (minimum %d characters)", minAPIKeyLength)
	}

	// Enforce maximum key length to prevent DoS
	if len(key) > maxAPIKeyLength {
		return fmt.Errorf("API key too long (maximum %d characters)", maxAPIKeyLength)
	}

	// Check for common insecure patterns
	if strings.Contains(key, " ") {
		return errors.New("API key contains spaces")
	}

	if strings.Contains(key, "${") {
		return errors.New("API key contains unexpanded template variables")
	}

	// Check for obvious test keys
	lowerKey := strings.ToLower(key)
	if strings.Contains(lowerKey, "test") ||
	   strings.Contains(lowerKey, "demo") ||
	   strings.Contains(lowerKey, "example") ||
	   strings.Contains(lowerKey, "sk-1234") ||
	   strings.Contains(lowerKey, "your-api-key") {
		return errors.New("API key appears to be a test/demo key")
	}

	// Check for base64-like patterns (most API keys are base64)
	if !isValidKeyFormat(key) {
		return errors.New("API key format appears invalid")
	}

	return nil
}

// isValidKeyFormat checks if the key follows common API key patterns
func isValidKeyFormat(key string) bool {
	// Most API keys are alphanumeric with specific separators
	// This is a basic check - adjust based on your API provider's format

	// Check for reasonable character distribution
	if len(key) < 10 {
		return false
	}

	// Count different character types
	var letters, numbers, special int
	for _, char := range key {
		switch {
		case (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z'):
			letters++
		case char >= '0' && char <= '9':
			numbers++
		case char == '-' || char == '_' || char == '.':
			special++
		default:
			return false // Invalid character
		}
	}

	// Must have at least some letters and numbers
	return letters > 0 && numbers > 0
}
