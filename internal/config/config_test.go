package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultConfigWithEnvOverrides(t *testing.T) {
	t.Setenv(envAPIKey, "sk-abc123def456ghi789jkl012mno345pqr")
	t.Setenv(envAPIURL, "https://example.com")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.API.URL != "https://example.com" {
		t.Fatalf("expected API URL override, got %q", cfg.API.URL)
	}

	if cfg.API.Key != "sk-abc123def456ghi789jkl012mno345pqr" {
		t.Fatalf("expected API key override, got %q", cfg.API.Key)
	}

	if cfg.Model.Name != "groq/moonshotai/kimi-k2-instruct-0905" {
		t.Fatalf("expected default model name, got %q", cfg.Model.Name)
	}
}

func TestLoad_FromFile(t *testing.T) {
	t.Setenv(envAPIKey, "")
	t.Setenv(envAPIURL, "")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  url: https://api.test/v1\n  key: sk-abc123def456ghi789jkl012mno345pqr\nmodel:\n  name: gpt-test\n  temperature: 0.5\n  stream: false\n")

	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.API.URL != "https://api.test/v1" {
		t.Errorf("expected API URL %q, got %q", "https://api.test/v1", cfg.API.URL)
	}
	if cfg.API.Key != "sk-abc123def456ghi789jkl012mno345pqr" {
		t.Errorf("expected API key %q, got %q", "sk-abc123def456ghi789jkl012mno345pqr", cfg.API.Key)
	}
	if cfg.Model.Name != "gpt-test" {
		t.Errorf("expected model name %q, got %q", "gpt-test", cfg.Model.Name)
	}
	if cfg.Model.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.Model.Temperature)
	}
	if cfg.Model.Stream != false {
		t.Errorf("expected stream false, got %t", cfg.Model.Stream)
	}
}

func TestLoad_InvalidTemperature(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  url: https://api.test/v1\n  key: test-token\nmodel:\n  name: gpt-test\n  temperature: 5\n")

	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid temperature, got none")
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	// Ensure no environment fallback is present.
	t.Setenv(envAPIKey, "")
	t.Setenv(envAPIURL, "")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  url: https://api.test/v1\nmodel:\n  name: gpt-test\n  temperature: 0.5\n")

	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing API key, got none")
	}
}
