package config

import (
	"bufio"
	"bytes"
	"encoding/json"
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
}

// Load reads configuration from ~/.config/affinity/config.json (same path as the affinity CLI).
// Legacy key=value content in that file is still supported if the file is not JSON.
// Environment variables always override file values.
func Load() *Config {
	fileVals := loadConfigFile(affinityConfigPath())

	c := &Config{
		BaseURL:            envOrFile("AFFINE_BASE_URL", fileVals, "baseUrl", "http://localhost:3010"),
		GraphQLPath:        envOrFile("AFFINE_GRAPHQL_PATH", fileVals, "graphqlPath", "/graphql"),
		APIToken:           envOrFile("AFFINE_API_TOKEN", fileVals, "apiToken", ""),
		Cookie:             envOrFile("AFFINE_COOKIE", fileVals, "cookie", ""),
		Email:              envOrFile("AFFINE_EMAIL", fileVals, "email", ""),
		Password:           envOrFile("AFFINE_PASSWORD", fileVals, "password", ""),
		DefaultWorkspaceID: envOrFile("AFFINE_WORKSPACE_ID", fileVals, "defaultWorkspaceId", ""),
		HeadersJSON:        envOrFile("AFFINE_HEADERS_JSON", fileVals, "headersJson", ""),
		WSClientVersion:    envOrDefault("AFFINE_WS_CLIENT_VERSION", fileOrDefault(fileVals, "wsClientVersion", "0.26.0")),
	}

	c.BaseURL = normalizeURL(c.BaseURL)
	return c
}

func affinityConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "affinity", "config.json")
}

func fileOrDefault(fileVals map[string]string, fileKey, defaultVal string) string {
	if v, ok := fileVals[fileKey]; ok && v != "" {
		return v
	}
	return defaultVal
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

func loadConfigFile(path string) map[string]string {
	if path == "" {
		return map[string]string{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	trim := bytes.TrimSpace(data)
	if len(trim) > 0 && trim[0] == '{' {
		var obj map[string]any
		if json.Unmarshal(trim, &obj) == nil {
			return mergeAffinityJSON(obj)
		}
	}
	return parseKeyValueConfig(data)
}

// mergeAffinityJSON maps ~/.config/affinity/config.json keys into the same map shape as key=value files.
func mergeAffinityJSON(raw map[string]any) map[string]string {
	out := make(map[string]string)
	set := func(envKey, camelKey, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		out[envKey] = val
		if camelKey != "" {
			out[camelKey] = val
		}
	}
	for k, v := range raw {
		s := stringifyJSONConfigVal(v)
		if s == "" {
			continue
		}
		switch k {
		case "url":
			set("AFFINE_BASE_URL", "baseUrl", strings.TrimRight(s, "/"))
		case "workspace_id":
			set("AFFINE_WORKSPACE_ID", "defaultWorkspaceId", s)
		case "email":
			set("AFFINE_EMAIL", "email", s)
		case "password":
			set("AFFINE_PASSWORD", "password", s)
		case "session_token":
			set("AFFINE_COOKIE", "cookie", s)
		case "api_token":
			set("AFFINE_API_TOKEN", "apiToken", s)
		case "graphql_path":
			set("AFFINE_GRAPHQL_PATH", "graphqlPath", s)
		case "headers_json":
			set("AFFINE_HEADERS_JSON", "headersJson", s)
		case "ws_client_version":
			out["wsClientVersion"] = s
		default:
			if strings.HasPrefix(k, "AFFINE_") {
				out[k] = s
			}
		}
	}
	return out
}

func stringifyJSONConfigVal(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%.0f", x)
		}
		return strings.TrimSpace(fmt.Sprint(x))
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func parseKeyValueConfig(data []byte) map[string]string {
	vals := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			vals[strings.TrimSpace(parts[0])] = v
		}
	}
	return vals
}

func envOrFile(envKey string, fileVals map[string]string, fileKey, defaultVal string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
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
