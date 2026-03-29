package graphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want 'Bearer test-token'", r.Header.Get("Authorization"))
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Query != "query { workspaces { id } }" {
			t.Errorf("Query = %q", req.Query)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{
			Data: json.RawMessage(`{"workspaces":[{"id":"ws-1"}]}`),
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-token", "", nil)
	data, err := c.Request(context.Background(), "query { workspaces { id } }", nil)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	var result struct {
		Workspaces []struct {
			ID string `json:"id"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(result.Workspaces) != 1 || result.Workspaces[0].ID != "ws-1" {
		t.Errorf("unexpected result: %s", string(data))
	}
}

func TestRequestWithCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=abc123" {
			t.Errorf("Cookie = %q, want 'session=abc123'", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Data: json.RawMessage(`{}`)})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "session=abc123", nil)
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
}

func TestRequestWithExtraHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "hello" {
			t.Errorf("X-Custom = %q, want 'hello'", r.Header.Get("X-Custom"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Data: json.RawMessage(`{}`)})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", map[string]string{"X-Custom": "hello"})
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
}

func TestRequestGraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{
			Errors: []gqlError{{Message: "not found"}},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", nil)
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "graphql error: not found" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", nil)
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRequestNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>Cloudflare challenge</html>"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", nil)
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error for non-JSON response")
	}
}

func TestRequestWithVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		json.NewDecoder(r.Body).Decode(&req)
		if req.Variables == nil {
			t.Error("Variables should not be nil")
		}
		if req.Variables["id"] != "ws-123" {
			t.Errorf("Variables[id] = %v, want ws-123", req.Variables["id"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Data: json.RawMessage(`{"workspace":{"id":"ws-123"}}`)})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", nil)
	_, err := c.Request(context.Background(), "query($id:String!){ workspace(id:$id) { id } }",
		map[string]any{"id": "ws-123"})
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
}

func TestExtraHeadersReturnsCopy(t *testing.T) {
	c := NewClient("http://example", "token", "c=1", map[string]string{"X-A": "1", "X-B": "2"})
	h := c.ExtraHeaders()
	if len(h) != 2 || h["X-A"] != "1" {
		t.Fatalf("ExtraHeaders = %v", h)
	}
	h["X-A"] = "mutated"
	h2 := c.ExtraHeaders()
	if h2["X-A"] != "1" {
		t.Error("mutating returned map affected client internals")
	}
}

func TestSetCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "new-cookie" {
			t.Errorf("Cookie = %q, want 'new-cookie'", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Data: json.RawMessage(`{}`)})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "old-cookie", nil)
	c.SetCookie("new-cookie")
	_, err := c.Request(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"ab", 2, "ab"},
		{"abc", 2, "ab..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}
