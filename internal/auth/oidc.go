package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type oauthPayload struct {
	State string `json:"state"`
	Code  string `json:"code"`
}

func SignInOIDC(ctx context.Context, baseURL, version string) error {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is required for OIDC sign-in")
	}
	dir := filepath.Join(runtimeDir, "affine-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "oauth.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("another OIDC sign-in is already running")
		}
		return fmt.Errorf("create OIDC sign-in lock: %w", err)
	}
	lock.Close()
	defer os.Remove(filepath.Join(dir, "oauth.lock"))
	callbackPath := filepath.Join(dir, "oauth-callback")
	os.Remove(callbackPath)

	cleanupCallback, err := installCallbackHandler()
	if err != nil {
		return err
	}
	defer cleanupCallback()
	nonce, err := randomToken()
	if err != nil {
		return err
	}
	authURL, err := oauthPreflight(ctx, baseURL, version, nonce)
	if err != nil {
		return err
	}
	if err := openBrowser(authURL); err != nil {
		return fmt.Errorf("open OIDC authorization: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Complete OIDC authorization in the browser...")
	callback, err := waitForCallback(ctx, callbackPath)
	if err != nil {
		return err
	}
	payload, err := parseCallback(callback)
	if err != nil {
		return err
	}
	exchangeCode, err := oauthCallback(ctx, baseURL, version, nonce, payload)
	if err != nil {
		return err
	}
	response, err := exchangeSession(ctx, baseURL, version, exchangeCode)
	if err != nil {
		return err
	}
	return StoreSession(baseURL, response)
}

func openBrowser(rawURL string) error {
	for _, command := range [][]string{{"gio", "open"}, {"xdg-open"}} {
		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}
		if err := exec.Command(command[0], append(command[1:], rawURL)...).Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("neither gio nor xdg-open could open the browser")
}

func DeliverOIDCCallback(rawURL string) error {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is required")
	}
	dir := filepath.Join(runtimeDir, "affine-cli")
	if _, err := os.Stat(filepath.Join(dir, "oauth.lock")); err != nil {
		return fmt.Errorf("no OIDC sign-in is waiting")
	}
	temp := filepath.Join(dir, "oauth-callback.tmp")
	if err := os.WriteFile(temp, []byte(rawURL), 0o600); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(dir, "oauth-callback"))
}

func installCallbackHandler() (func(), error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	apps := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return nil, err
	}
	const scheme = "x-scheme-handler/affine-beta"
	const desktop = "affine-cli-oauth.desktop"
	previousBytes, _ := exec.Command("xdg-mime", "query", "default", scheme).Output()
	previous := strings.TrimSpace(string(previousBytes))
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=AFFiNE CLI OAuth
NoDisplay=true
Exec=%s user oauth-callback %%u
MimeType=%s;
`, strconv.Quote(executable), scheme)
	entryPath := filepath.Join(apps, desktop)
	if err := os.WriteFile(entryPath, []byte(entry), 0o644); err != nil {
		return nil, err
	}
	if err := updateDesktopDatabase(apps); err != nil {
		return nil, fmt.Errorf("update desktop application database: %w", err)
	}
	if err := exec.Command("xdg-mime", "default", desktop, scheme).Run(); err != nil {
		return nil, fmt.Errorf("register OIDC callback: %w", err)
	}
	return func() {
		if previous != "" {
			_ = exec.Command("xdg-mime", "default", previous, scheme).Run()
		} else {
			removeMimeDefault(home, scheme, desktop)
		}
		_ = os.Remove(entryPath)
		_ = updateDesktopDatabase(apps)
	}, nil
}

func updateDesktopDatabase(apps string) error {
	if _, err := exec.LookPath("update-desktop-database"); err != nil {
		return nil
	}
	return exec.Command("update-desktop-database", apps).Run()
}

func removeMimeDefault(home, scheme, desktop string) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	path := filepath.Join(configHome, "mimeapps.list")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	prefix := scheme + "="
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		var kept []string
		for _, value := range strings.Split(strings.TrimPrefix(line, prefix), ";") {
			if value != "" && value != desktop {
				kept = append(kept, value)
			}
		}
		if len(kept) == 0 {
			lines[i] = ""
		} else {
			lines[i] = prefix + strings.Join(kept, ";") + ";"
		}
		changed = true
	}
	if changed {
		_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
	}
}

func oauthPreflight(ctx context.Context, baseURL, version, nonce string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"provider":     "OIDC",
		"client":       "affine-beta",
		"client_nonce": nonce,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/oauth/preflight", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	setNativeHeaders(req, version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || result.URL == "" {
		return "", fmt.Errorf("OIDC preflight failed (HTTP %d)", resp.StatusCode)
	}
	return result.URL, nil
}

func waitForCallback(ctx context.Context, path string) (string, error) {
	timeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-timeout.Done():
			return "", fmt.Errorf("OIDC authorization timed out")
		case <-ticker.C:
			data, err := os.ReadFile(path)
			if err == nil {
				os.Remove(path)
				return string(data), nil
			}
			if !os.IsNotExist(err) {
				return "", err
			}
		}
	}
}

func parseCallback(raw string) (oauthPayload, error) {
	callback, err := url.Parse(raw)
	if err != nil {
		return oauthPayload{}, err
	}
	if callback.Scheme != "affine-beta" || callback.Host != "authentication" || callback.Query().Get("method") != "oauth" {
		return oauthPayload{}, fmt.Errorf("invalid OIDC callback")
	}
	var payload oauthPayload
	if err := json.Unmarshal([]byte(callback.Query().Get("payload")), &payload); err != nil {
		return oauthPayload{}, err
	}
	if payload.State == "" || payload.Code == "" {
		return oauthPayload{}, fmt.Errorf("incomplete OIDC callback")
	}
	return payload, nil
}

func oauthCallback(ctx context.Context, baseURL, version, nonce string, payload oauthPayload) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"code":         payload.Code,
		"state":        payload.State,
		"client_nonce": nonce,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/oauth/callback", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	setNativeHeaders(req, version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		ExchangeCode string `json:"exchangeCode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || result.ExchangeCode == "" {
		return "", fmt.Errorf("OIDC callback failed (HTTP %d)", resp.StatusCode)
	}
	return result.ExchangeCode, nil
}

func exchangeSession(ctx context.Context, baseURL, version, code string) (tokenResponse, error) {
	installationID, err := installationID()
	if err != nil {
		return tokenResponse{}, err
	}
	hostname, _ := os.Hostname()
	body, _ := json.Marshal(map[string]string{
		"code":           code,
		"installationId": installationID,
		"platform":       "electron",
		"deviceName":     hostname,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/auth/session/exchange", strings.NewReader(string(body)))
	if err != nil {
		return tokenResponse{}, err
	}
	setNativeHeaders(req, version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()
	var result tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("OIDC session exchange failed (HTTP %d)", resp.StatusCode)
	}
	return result, nil
}

func installationID() (string, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "installation-id")
	if value, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(value)), nil
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	value := fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		return "", err
	}
	return value, nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
