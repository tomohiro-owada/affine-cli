package workspacetitles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMergeDocListTitles(t *testing.T) {
	raw := json.RawMessage(`{"workspace":{"docs":{"edges":[{"node":{"id":"abc","title":null}},{"node":{"id":"def","title":""}},{"node":{"id":"ghi","title":"Keep"}}]}}}`)
	titles := map[string]string{"abc": "From Yjs", "def": "Also Yjs", "ghi": "Ignored"}

	out := MergeDocListTitles(raw, titles)
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}

	ws := got["workspace"].(map[string]any)
	docs := ws["docs"].(map[string]any)
	edges := docs["edges"].([]any)
	n0 := edges[0].(map[string]any)["node"].(map[string]any)
	n1 := edges[1].(map[string]any)["node"].(map[string]any)
	n2 := edges[2].(map[string]any)["node"].(map[string]any)

	if n0["title"] != "From Yjs" {
		t.Errorf("abc title = %v", n0["title"])
	}
	if n1["title"] != "Also Yjs" {
		t.Errorf("def title = %v", n1["title"])
	}
	if n2["title"] != "Keep" {
		t.Errorf("ghi title should stay Keep, got %v", n2["title"])
	}
}

func TestGetBody_extraHeadersPrecedence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Proxy-Auth") != "gateway-token" {
			t.Errorf("X-Proxy-Auth = %q", r.Header.Get("X-Proxy-Auth"))
		}
		if r.Header.Get("Cookie") != "affine_session=abc" {
			t.Errorf("Cookie = %q", r.Header.Get("Cookie"))
		}
		if r.Header.Get("Authorization") != "Bearer api" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	body, err := getBody(context.Background(), http.DefaultClient, srv.URL, "affine_session=abc", "api",
		map[string]string{"X-Proxy-Auth": "gateway-token"})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}
