package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	BaseURL            string
	GraphQLPath        string
	APIToken           string
	Cookie             string
	Email              string
	Password           string
	DefaultWorkspaceID string
	HeadersJSON        string
	WSClientVersion    string
	OIDCSession        bool
}

func Load() *Config {
	fileVals := loadConfigFile()

	c := &Config{
		BaseURL:            envOrFile("AFFINE_BASE_URL", fileVals, "baseUrl", "http://localhost:3010"),
		GraphQLPath:        envOrFile("AFFINE_GRAPHQL_PATH", fileVals, "graphqlPath", "/graphql"),
		APIToken:           envOrFile("AFFINE_API_TOKEN", fileVals, "apiToken", ""),
		Cookie:             envOrFile("AFFINE_COOKIE", fileVals, "cookie", ""),
		Email:              envOrFile("AFFINE_EMAIL", fileVals, "email", ""),
		Password:           envOrFile("AFFINE_PASSWORD", fileVals, "password", ""),
		DefaultWorkspaceID: envOrFile("AFFINE_WORKSPACE_ID", fileVals, "defaultWorkspaceId", ""),
		HeadersJSON:        os.Getenv("AFFINE_HEADERS_JSON"),
		WSClientVersion:    envOrDefault("AFFINE_WS_CLIENT_VERSION", "0.27.4"),
	}

	c.BaseURL = normalizeURL(c.BaseURL)
	return c
}

func (c *Config) GraphQLEndpoint() string {
	return c.BaseURL + c.GraphQLPath
}

func (c *Config) WSEndpoint() string {
	u := c.BaseURL + c.GraphQLPath
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u
}

func loadConfigFile() map[string]string {
	vals := make(map[string]string)

	home, err := os.UserHomeDir()
	if err != nil {
		return vals
	}

	path := filepath.Join(home, ".config", "affine-cli", "config")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		f, err = os.Open(filepath.Join(home, ".config", "affine-mcp", "config"))
	}
	if err != nil {
		return vals
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			vals[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return vals
}

func envOrFile(envKey string, fileVals map[string]string, fileKey, defaultVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	// Config file can use either the env var name or the camelCase key
	if v, ok := fileVals[envKey]; ok && v != "" {
		return v
	}
	if v, ok := fileVals[fileKey]; ok && v != "" {
		return v
	}
	return defaultVal
}

func envOrDefault(envKey, defaultVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		fmt.Fprintf(os.Stderr, "Warning: URL contains credentials, stripping them\n")
		u.User = nil
	}
	result := u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/")
	return result
}
