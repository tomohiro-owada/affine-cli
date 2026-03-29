// Package workspacetitles loads per-page data from AFFiNE workspace Yjs
// snapshots (HTTP /api/workspaces/.../docs/...).
package workspacetitles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tomohiro-owada/affine-cli/internal/yjs"
)

type loadedYjs struct {
	eng        *yjs.Engine
	wsDocID    int
	propsDocID int
}

func loadWorkspaceYjs(ctx context.Context, baseURL, workspaceID, cookie, bearer string, extraHeaders map[string]string) (*loadedYjs, error) {
	if cookie == "" && bearer == "" {
		return nil, fmt.Errorf("no session cookie or API token: cannot load workspace Yjs snapshot")
	}
	base := strings.TrimRight(baseURL, "/")
	cli := &http.Client{Timeout: 60 * time.Second}

	wsURL := fmt.Sprintf("%s/api/workspaces/%s/docs/%s", base, workspaceID, workspaceID)
	body, err := getBody(ctx, cli, wsURL, cookie, bearer, extraHeaders)
	if err != nil {
		return nil, fmt.Errorf("workspace snapshot: %w", err)
	}

	eng, err := yjs.NewEngine()
	if err != nil {
		return nil, err
	}
	wsDocID, err := eng.ApplyUpdate(body)
	if err != nil {
		return nil, fmt.Errorf("apply workspace yjs: %w", err)
	}

	propsDocID := -1
	propsURL := fmt.Sprintf("%s/api/workspaces/%s/docs/db$%s$docProperties", base, workspaceID, workspaceID)
	propsBody, err := getBody(ctx, cli, propsURL, cookie, bearer, extraHeaders)
	if err == nil && len(propsBody) > 0 {
		pid, perr := eng.ApplyUpdate(propsBody)
		if perr == nil {
			propsDocID = pid
		}
	}

	return &loadedYjs{eng: eng, wsDocID: wsDocID, propsDocID: propsDocID}, nil
}

// FetchPageTitles downloads the workspace index and docProperties docs and returns
// page ID → title. cookie and/or bearer are sent like the AFFiNE web app.
// extraHeaders should match the GraphQL client (e.g. reverse-proxy auth from AFFINE_HEADERS_JSON).
func FetchPageTitles(ctx context.Context, baseURL, workspaceID, cookie, bearer string, extraHeaders map[string]string) (map[string]string, error) {
	w, err := loadWorkspaceYjs(ctx, baseURL, workspaceID, cookie, bearer, extraHeaders)
	if err != nil {
		return nil, err
	}
	return w.eng.WorkspacePageTitles(w.wsDocID, w.propsDocID)
}

// FetchPageList returns a JSON array of pages with id, title, parentId, createDate (non-trash only).
func FetchPageList(ctx context.Context, baseURL, workspaceID, cookie, bearer string, extraHeaders map[string]string) (json.RawMessage, error) {
	w, err := loadWorkspaceYjs(ctx, baseURL, workspaceID, cookie, bearer, extraHeaders)
	if err != nil {
		return nil, err
	}
	raw, err := w.eng.WorkspacePageListJSON(w.wsDocID, w.propsDocID)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func getBody(ctx context.Context, cli *http.Client, urlStr, cookie, bearer string, extraHeaders map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range extraHeaders {
		if k != "" && v != "" {
			req.Header.Set(k, v)
		}
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("HTTP 404")
	}
	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	return io.ReadAll(resp.Body)
}

// MergeDocListTitles walks a ListDocs GraphQL JSON payload and fills each node's
// title from titles when GraphQL left it null or empty.
func MergeDocListTitles(data json.RawMessage, titles map[string]string) json.RawMessage {
	if len(titles) == 0 {
		return data
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return data
	}
	walkMerge(root, titles)
	out, err := json.Marshal(root)
	if err != nil {
		return data
	}
	return out
}

func walkMerge(v any, titles map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		if node, ok := x["node"].(map[string]any); ok {
			id, _ := node["id"].(string)
			if id != "" {
				if t, ok := titles[id]; ok && t != "" {
					mergeTitle(node, t)
				}
			}
		}
		for _, child := range x {
			walkMerge(child, titles)
		}
	case []any:
		for _, item := range x {
			walkMerge(item, titles)
		}
	}
}

func mergeTitle(node map[string]any, yjsTitle string) {
	cur := node["title"]
	if cur == nil {
		node["title"] = yjsTitle
		return
	}
	s, ok := cur.(string)
	if !ok || strings.TrimSpace(s) == "" {
		node["title"] = yjsTitle
	}
}
