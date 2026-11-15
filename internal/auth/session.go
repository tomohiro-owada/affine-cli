package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type Session struct {
	Version          int            `json:"version"`
	TokenType        string         `json:"tokenType"`
	AccessToken      string         `json:"accessToken"`
	AccessExpiresAt  string         `json:"accessExpiresAt"`
	RefreshToken     string         `json:"refreshToken"`
	RefreshExpiresAt string         `json:"refreshExpiresAt"`
	Session          SessionDetails `json:"session"`
}

type SessionDetails struct {
	ID                string `json:"id"`
	AbsoluteExpiresAt string `json:"absoluteExpiresAt"`
}

type tokenResponse struct {
	TokenType        string         `json:"tokenType"`
	AccessToken      string         `json:"accessToken"`
	ExpiresIn        int64          `json:"expiresIn"`
	RefreshToken     string         `json:"refreshToken"`
	RefreshExpiresAt string         `json:"refreshExpiresAt"`
	Session          SessionDetails `json:"session"`
}

func sessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "affine-cli"), nil
}

func withSessionLock(fn func(string) error) error {
	dir, err := sessionDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "session.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN) //nolint:errcheck
	return fn(dir)
}

func loadSessions(dir string) (map[string]Session, error) {
	data, err := os.ReadFile(filepath.Join(dir, "sessions.json"))
	if os.IsNotExist(err) {
		return map[string]Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions map[string]Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parse OIDC sessions: %w", err)
	}
	if sessions == nil {
		sessions = map[string]Session{}
	}
	return sessions, nil
}

func saveSessions(dir string, sessions map[string]Session) error {
	data, err := json.Marshal(sessions)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "sessions.json")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temp, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func sessionFromResponse(response tokenResponse, now time.Time) (Session, error) {
	session := Session{
		Version:          1,
		TokenType:        response.TokenType,
		AccessToken:      response.AccessToken,
		AccessExpiresAt:  now.Add(time.Duration(response.ExpiresIn) * time.Second).UTC().Format(time.RFC3339Nano),
		RefreshToken:     response.RefreshToken,
		RefreshExpiresAt: response.RefreshExpiresAt,
		Session:          response.Session,
	}
	if session.TokenType != "Bearer" || session.AccessToken == "" || session.RefreshToken == "" || response.ExpiresIn <= 0 {
		return Session{}, fmt.Errorf("invalid token response")
	}
	return session, nil
}

func StoreSession(baseURL string, response tokenResponse) error {
	return withSessionLock(func(dir string) error {
		sessions, err := loadSessions(dir)
		if err != nil {
			return err
		}
		session, err := sessionFromResponse(response, time.Now())
		if err != nil {
			return err
		}
		sessions[strings.TrimRight(baseURL, "/")] = session
		return saveSessions(dir, sessions)
	})
}

func SessionAccessToken(ctx context.Context, baseURL, version string) (string, error) {
	var token string
	err := withSessionLock(func(dir string) error {
		sessions, err := loadSessions(dir)
		if err != nil {
			return err
		}
		key := strings.TrimRight(baseURL, "/")
		session, ok := sessions[key]
		if !ok {
			return nil
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, session.AccessExpiresAt)
		if err != nil {
			return fmt.Errorf("invalid access token expiry: %w", err)
		}
		if time.Until(expiresAt) <= 2*time.Minute {
			refreshed, err := refreshSession(ctx, key, version, session.RefreshToken)
			if err != nil {
				return err
			}
			session, err = sessionFromResponse(refreshed, time.Now())
			if err != nil {
				return err
			}
			sessions[key] = session
			if err := saveSessions(dir, sessions); err != nil {
				return err
			}
		}
		token = session.AccessToken
		return nil
	})
	return token, err
}

func refreshSession(ctx context.Context, baseURL, version, refreshToken string) (tokenResponse, error) {
	body, _ := json.Marshal(map[string]string{"refreshToken": refreshToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/auth/session/refresh", strings.NewReader(string(body)))
	if err != nil {
		return tokenResponse{}, err
	}
	setNativeHeaders(req, version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()
	var response tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return tokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("OIDC refresh failed (HTTP %d)", resp.StatusCode)
	}
	return response, nil
}

func setNativeHeaders(req *http.Request, version string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-affine-client-kind", "native")
	req.Header.Set("x-affine-version", version)
}
