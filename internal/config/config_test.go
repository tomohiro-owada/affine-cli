package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear all env vars to test defaults
	for _, key := range []string{
		"AFFINE_BASE_URL", "AFFINE_GRAPHQL_PATH", "AFFINE_API_TOKEN",
		"AFFINE_COOKIE", "AFFINE_EMAIL", "AFFINE_PASSWORD",
		"AFFINE_WORKSPACE_ID", "AFFINE_HEADERS_JSON", "AFFINE_WS_CLIENT_VERSION",
	} {
		t.Setenv(key, "")
	}

	cfg := Load()
	if cfg.BaseURL != "http://localhost:3010" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:3010")
	}
	if cfg.GraphQLPath != "/graphql" {
		t.Errorf("GraphQLPath = %q, want %q", cfg.GraphQLPath, "/graphql")
	}
	if cfg.WSClientVersion != "0.27.4" {
		t.Errorf("WSClientVersion = %q, want %q", cfg.WSClientVersion, "0.27.4")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("AFFINE_BASE_URL", "https://test.example.com")
	t.Setenv("AFFINE_API_TOKEN", "test-token-123")
	t.Setenv("AFFINE_WORKSPACE_ID", "ws-abc")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/gql")

	cfg := Load()
	if cfg.BaseURL != "https://test.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://test.example.com")
	}
	if cfg.APIToken != "test-token-123" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "test-token-123")
	}
	if cfg.DefaultWorkspaceID != "ws-abc" {
		t.Errorf("DefaultWorkspaceID = %q, want %q", cfg.DefaultWorkspaceID, "ws-abc")
	}
	if cfg.GraphQLPath != "/gql" {
		t.Errorf("GraphQLPath = %q, want %q", cfg.GraphQLPath, "/gql")
	}
}

func TestGraphQLEndpoint(t *testing.T) {
	t.Setenv("AFFINE_BASE_URL", "https://affine.example.com")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/graphql")

	cfg := Load()
	want := "https://affine.example.com/graphql"
	if got := cfg.GraphQLEndpoint(); got != want {
		t.Errorf("GraphQLEndpoint() = %q, want %q", got, want)
	}
}

func TestWSEndpoint(t *testing.T) {
	t.Setenv("AFFINE_BASE_URL", "https://affine.example.com")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/graphql")

	cfg := Load()
	want := "wss://affine.example.com/graphql"
	if got := cfg.WSEndpoint(); got != want {
		t.Errorf("WSEndpoint() = %q, want %q", got, want)
	}
}

func TestWSEndpointHTTP(t *testing.T) {
	t.Setenv("AFFINE_BASE_URL", "http://localhost:3010")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/graphql")

	cfg := Load()
	want := "ws://localhost:3010/graphql"
	if got := cfg.WSEndpoint(); got != want {
		t.Errorf("WSEndpoint() = %q, want %q", got, want)
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/", "https://example.com"},
		{"https://example.com/path/", "https://example.com/path"},
		{"http://localhost:3010", "http://localhost:3010"},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "affine-cli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config")
	content := "AFFINE_BASE_URL=https://from-file.example.com\napiToken=file-token-xyz\n# comment line\n\ndefaultWorkspaceId=ws-from-file\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME so loadConfigFile finds our temp file
	t.Setenv("HOME", tmpDir)
	// Clear env vars so file values take effect
	t.Setenv("AFFINE_BASE_URL", "")
	t.Setenv("AFFINE_API_TOKEN", "")
	t.Setenv("AFFINE_WORKSPACE_ID", "")

	cfg := Load()
	if cfg.BaseURL != "https://from-file.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://from-file.example.com")
	}
	if cfg.APIToken != "file-token-xyz" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "file-token-xyz")
	}
	if cfg.DefaultWorkspaceID != "ws-from-file" {
		t.Errorf("DefaultWorkspaceID = %q, want %q", cfg.DefaultWorkspaceID, "ws-from-file")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "affine-cli")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.WriteFile(filepath.Join(configDir, "config"), []byte("AFFINE_BASE_URL=https://file.example.com\n"), 0o644)

	t.Setenv("HOME", tmpDir)
	t.Setenv("AFFINE_BASE_URL", "https://env.example.com")

	cfg := Load()
	if cfg.BaseURL != "https://env.example.com" {
		t.Errorf("env should override file: BaseURL = %q, want %q", cfg.BaseURL, "https://env.example.com")
	}
}
