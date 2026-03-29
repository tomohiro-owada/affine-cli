package config

import (
	"os"
	"path/filepath"
	"testing"
)

func affinityConfigPathUnderHome(home string) string {
	return filepath.Join(home, ".config", "affinity", "config.json")
}

func TestLoadDefaults(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	for _, key := range []string{
		"AFFINE_BASE_URL", "AFFINE_GRAPHQL_PATH", "AFFINE_API_TOKEN",
		"AFFINE_COOKIE", "AFFINE_EMAIL", "AFFINE_PASSWORD",
		"AFFINE_WORKSPACE_ID", "AFFINE_HEADERS_JSON", "AFFINE_WS_CLIENT_VERSION",
		"AFFINE_CONFIG_FILE",
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
	if cfg.WSClientVersion != "0.26.0" {
		t.Errorf("WSClientVersion = %q, want %q", cfg.WSClientVersion, "0.26.0")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AFFINE_CONFIG_FILE", "")
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
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AFFINE_CONFIG_FILE", "")
	t.Setenv("AFFINE_BASE_URL", "https://affine.example.com")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/graphql")

	cfg := Load()
	want := "https://affine.example.com/graphql"
	if got := cfg.GraphQLEndpoint(); got != want {
		t.Errorf("GraphQLEndpoint() = %q, want %q", got, want)
	}
}

func TestWSEndpoint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AFFINE_CONFIG_FILE", "")
	t.Setenv("AFFINE_BASE_URL", "https://affine.example.com")
	t.Setenv("AFFINE_GRAPHQL_PATH", "/graphql")

	cfg := Load()
	want := "wss://affine.example.com/graphql"
	if got := cfg.WSEndpoint(); got != want {
		t.Errorf("WSEndpoint() = %q, want %q", got, want)
	}
}

func TestWSEndpointHTTP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AFFINE_CONFIG_FILE", "")
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

func TestLoadConfigAffinityJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".config", "affinity"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := affinityConfigPathUnderHome(tmpDir)
	content := `{
  "url": "https://from-file.example.com",
  "api_token": "file-token-xyz",
  "workspace_id": "ws-from-file",
  "graphql_path": "/gql"
}
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", tmpDir)
	t.Setenv("AFFINE_CONFIG_FILE", "")
	t.Setenv("AFFINE_BASE_URL", "")
	t.Setenv("AFFINE_API_TOKEN", "")
	t.Setenv("AFFINE_WORKSPACE_ID", "")
	t.Setenv("AFFINE_GRAPHQL_PATH", "")

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
	if cfg.GraphQLPath != "/gql" {
		t.Errorf("GraphQLPath = %q, want %q", cfg.GraphQLPath, "/gql")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".config", "affinity"), 0o755)
	_ = os.WriteFile(affinityConfigPathUnderHome(tmpDir), []byte(`{"url": "https://file.example.com"}`), 0o644)

	t.Setenv("HOME", tmpDir)
	t.Setenv("AFFINE_CONFIG_FILE", "")
	t.Setenv("AFFINE_BASE_URL", "https://env.example.com")

	cfg := Load()
	if cfg.BaseURL != "https://env.example.com" {
		t.Errorf("env should override file: BaseURL = %q, want %q", cfg.BaseURL, "https://env.example.com")
	}
}

// AFFINE_CONFIG_FILE is ignored; only ~/.config/affinity/config.json is read.
func TestAFFINE_CONFIG_FILEIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".config", "affinity"), 0o755)
	custom := filepath.Join(tmpDir, "my-affine.conf")
	_ = os.WriteFile(custom, []byte(`{"url":"https://wrong.example.com"}`), 0o644)
	_ = os.WriteFile(affinityConfigPathUnderHome(tmpDir), []byte(`{"url":"https://affinity-path.example.com","workspace_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}`), 0o644)

	t.Setenv("HOME", tmpDir)
	t.Setenv("AFFINE_CONFIG_FILE", custom)
	t.Setenv("AFFINE_BASE_URL", "")
	t.Setenv("AFFINE_WORKSPACE_ID", "")

	cfg := Load()
	if cfg.BaseURL != "https://affinity-path.example.com" {
		t.Errorf("BaseURL = %q, want affinity config path only", cfg.BaseURL)
	}
	if cfg.DefaultWorkspaceID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("DefaultWorkspaceID = %q", cfg.DefaultWorkspaceID)
	}
}

func TestLegacyKeyValueFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".config", "affinity"), 0o755)
	cfgPath := affinityConfigPathUnderHome(tmpDir)
	_ = os.WriteFile(cfgPath, []byte("AFFINE_BASE_URL=https://legacy.example.com\n"), 0o644)

	t.Setenv("HOME", tmpDir)
	t.Setenv("AFFINE_CONFIG_FILE", "")
	t.Setenv("AFFINE_BASE_URL", "")

	cfg := Load()
	if cfg.BaseURL != "https://legacy.example.com" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
}
