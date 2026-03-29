package output

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/term"
)

// ErrorResponse is the structured JSON error format for agent consumption.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// isAgent returns true if stdout is NOT a terminal (piped to another process / agent).
func isAgent() bool {
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// JSON prints data as formatted JSON to stdout.
func JSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
	}
}

// RawJSON prints raw JSON bytes to stdout with indentation.
func RawJSON(data json.RawMessage) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Println(string(data))
		return
	}
	JSON(v)
}

// FilteredJSON outputs only the specified fields from a JSON object.
// It recursively unwraps single-key wrapper objects and handles
// AFFiNE's nested response patterns like workspace.docs.edges[].node.
func FilteredJSON(data json.RawMessage, fields []string) {
	if len(fields) == 0 {
		RawJSON(data)
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		RawJSON(data)
		return
	}

	// Recursively unwrap single-key wrapper objects
	unwrapped := unwrapToData(obj)

	switch v := unwrapped.(type) {
	case []any:
		filterArray(v, fields)
	case map[string]any:
		filtered := filterMap(v, fields)
		JSON(filtered)
	default:
		RawJSON(data)
	}
}

// unwrapToData recursively descends into single-key wrapper objects
// and extracts the inner data. Handles patterns like:
//   - {"workspace": {"docs": {"edges": [...]}}}  → extracts node objects from edges
//   - {"workspace": {"doc": {fields...}}}        → returns the doc object
//   - {"workspaces": [...]}                       → returns the array
func unwrapToData(obj map[string]any) any {
	// If this map has fields that look like leaf data (not a single wrapper), return it
	if len(obj) != 1 {
		return obj
	}
	for _, v := range obj {
		switch inner := v.(type) {
		case map[string]any:
			// Check for edges pattern (GraphQL pagination)
			if edges, ok := inner["edges"].([]any); ok {
				return extractNodes(edges)
			}
			return unwrapToData(inner)
		case []any:
			return inner
		default:
			return obj
		}
	}
	return obj
}

// extractNodes pulls "node" objects out of GraphQL edge arrays.
func extractNodes(edges []any) []any {
	var nodes []any
	for _, edge := range edges {
		if e, ok := edge.(map[string]any); ok {
			if node, ok := e["node"]; ok {
				nodes = append(nodes, node)
			} else {
				nodes = append(nodes, edge)
			}
		}
	}
	return nodes
}

func filterMap(obj map[string]any, fields []string) map[string]any {
	result := make(map[string]any)
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	for k, v := range obj {
		if fieldSet[k] {
			result[k] = v
		}
	}
	return result
}

func filterArray(arr []any, fields []string) {
	var results []any
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			results = append(results, filterMap(m, fields))
		}
	}
	JSON(results)
}

// Error prints an error as structured JSON to stderr (for agent consumption)
// or as plain text (for human consumption), and returns the error.
func Error(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if isAgent() {
		resp := ErrorResponse{Error: err.Error()}
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return err
}

// ErrorWithCode prints a structured error with error code.
func ErrorWithCode(code, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if isAgent() {
		resp := ErrorResponse{Error: err.Error(), Code: code}
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
	} else {
		fmt.Fprintf(os.Stderr, "Error [%s]: %v\n", code, err)
	}
	return err
}

// WarnWithCode writes a non-fatal warning to stderr. In agent mode (stdout not a TTY),
// emits JSON with the same error/code shape as ErrorWithCode so piped consumers stay consistent.
func WarnWithCode(code, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if isAgent() {
		resp := ErrorResponse{Error: msg, Code: code}
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp)
		return
	}
	fmt.Fprintf(os.Stderr, "Warning [%s]: %s\n", code, msg)
}

// DryRun prints what would happen without executing.
func DryRun(action string, details map[string]any) {
	if isAgent() {
		JSON(map[string]any{
			"dry_run": true,
			"action":  action,
			"details": details,
		})
	} else {
		fmt.Fprintf(os.Stderr, "[DRY RUN] %s\n", action)
		for k, v := range details {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", k, v)
		}
	}
}
