package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	content := `
# Sample comment
TEST_CRAWLER_VAR_A=hello_world
TEST_CRAWLER_VAR_B="quoted value"
TEST_CRAWLER_VAR_C='single quoted'
export TEST_CRAWLER_VAR_D=exported_value
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	loadEnvFile(envPath)

	if val := os.Getenv("TEST_CRAWLER_VAR_A"); val != "hello_world" {
		t.Errorf("expected hello_world, got %s", val)
	}
	if val := os.Getenv("TEST_CRAWLER_VAR_B"); val != "quoted value" {
		t.Errorf("expected 'quoted value', got %s", val)
	}
	if val := os.Getenv("TEST_CRAWLER_VAR_C"); val != "single quoted" {
		t.Errorf("expected 'single quoted', got %s", val)
	}
	if val := os.Getenv("TEST_CRAWLER_VAR_D"); val != "exported_value" {
		t.Errorf("expected 'exported_value', got %s", val)
	}
}
