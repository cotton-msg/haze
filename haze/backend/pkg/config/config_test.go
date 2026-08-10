package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestEnvStrSliceFromYAML(t *testing.T) {
	t.Setenv("CORS_TEST_ENV", "")
	cfg, err := Load(writeConfig(t, "cors:\n  allowed_origins:\n    - \"https://haze.app\"\n    - \"https://app.haze.app\"\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	origins := cfg.EnvStrSlice("CORS_TEST_ENV", "cors.allowed_origins", []string{"http://localhost:3000"})
	if len(origins) != 2 || origins[0] != "https://haze.app" || origins[1] != "https://app.haze.app" {
		t.Fatalf("unexpected origins: %v", origins)
	}
}

func TestEnvStrSliceFromEnvOverrides(t *testing.T) {
	t.Setenv("CORS_TEST_ENV", "http://a.com, http://b.com")
	cfg, err := Load(writeConfig(t, "cors:\n  allowed_origins:\n    - \"https://haze.app\"\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	origins := cfg.EnvStrSlice("CORS_TEST_ENV", "cors.allowed_origins", nil)
	if len(origins) != 2 || origins[0] != "http://a.com" || origins[1] != "http://b.com" {
		t.Fatalf("unexpected origins: %v", origins)
	}
}

func TestEnvStrSliceDefault(t *testing.T) {
	t.Setenv("CORS_TEST_ENV", "")
	cfg, err := Load(writeConfig(t, "server:\n  port: 8080\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	origins := cfg.EnvStrSlice("CORS_TEST_ENV", "cors.allowed_origins", []string{"http://localhost:3000"})
	if len(origins) != 1 || origins[0] != "http://localhost:3000" {
		t.Fatalf("unexpected origins: %v", origins)
	}
}
