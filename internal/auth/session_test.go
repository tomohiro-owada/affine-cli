package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionAccessTokenRefreshesAndPersistsRotation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "affine-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/session/refresh" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-affine-client-kind"); got != "native" {
			t.Fatalf("x-affine-client-kind = %q", got)
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{
			TokenType:        "Bearer",
			AccessToken:      "new-access",
			ExpiresIn:        600,
			RefreshToken:     "new-refresh",
			RefreshExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
			Session: SessionDetails{
				ID:                "session",
				AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	expired := Session{
		Version:          1,
		TokenType:        "Bearer",
		AccessToken:      "old-access",
		AccessExpiresAt:  time.Now().Add(-time.Minute).Format(time.RFC3339),
		RefreshToken:     "old-refresh",
		RefreshExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
		Session: SessionDetails{
			ID:                "session",
			AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		},
	}
	if err := saveSessions(dir, map[string]Session{server.URL: expired}); err != nil {
		t.Fatal(err)
	}

	token, err := SessionAccessToken(context.Background(), server.URL, "0.27.4")
	if err != nil {
		t.Fatal(err)
	}
	if token != "new-access" {
		t.Fatalf("token = %q", token)
	}
	sessions, err := loadSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[server.URL].RefreshToken != "new-refresh" {
		t.Fatal("rotated refresh token was not persisted")
	}
	info, err := os.Stat(filepath.Join(dir, "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %o", info.Mode().Perm())
	}
}
