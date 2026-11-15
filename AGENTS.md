# AFFiNE CLI — AI Agent Guide

## Overview
`affine` is a CLI for managing AFFiNE workspaces, documents, comments, blobs, and more. It supports both GraphQL-based metadata operations and WebSocket + Y.js CRDT document operations.

## Quick Start

```bash
# Set credentials (env vars or ~/.config/affine-cli/config)
export AFFINE_BASE_URL=https://your-affine-instance.com
export AFFINE_EMAIL=you@example.com
export AFFINE_PASSWORD=your-password
export AFFINE_WORKSPACE_ID=your-workspace-uuid

# Or use API token
export AFFINE_API_TOKEN=your-token
```

## Discovery

```bash
# List all commands and their parameters (machine-readable)
affine schema

# Check auth status
affine auth status
```

## Key Patterns

### JSON Output
All commands output structured JSON to stdout. Errors are output to stderr.

### Field Filtering
```bash
affine workspace list --fields id,public
affine doc list --fields id,title
```

### Dry Run
Destructive operations support `--dry-run`:
```bash
affine workspace delete -w UUID --dry-run
affine comment delete --id ID --dry-run
```

### JSON Input from Stdin
Complex inputs can be piped as JSON:
```bash
echo '{"doc-id":"abc","content":"hello"}' | affine comment create --json
```

### Workspace Scope
Most commands require a workspace ID via `-w` flag or `AFFINE_WORKSPACE_ID` env var.

## Command Reference

### Workspaces
- `affine workspace list` — List all workspaces
- `affine workspace get -w ID` — Get workspace details
- `affine workspace update -w ID --public --enable-ai` — Update settings
- `affine workspace delete -w ID` — Delete workspace

### Documents
- `affine doc list -w ID` — List docs (supports `--first`, `--offset`, `--after`)
- `affine doc get -w ID --doc-id DOC` — Get doc metadata
- `affine doc publish -w ID --doc-id DOC --mode Page` — Publish doc
- `affine doc revoke -w ID --doc-id DOC` — Revoke publication

### Comments
- `affine comment list -w ID --doc-id DOC`
- `affine comment create -w ID --doc-id DOC --content "text"`
- `affine comment update --id ID --content "new text"`
- `affine comment delete --id ID`
- `affine comment resolve --id ID --resolved`

### Tokens
- `affine token list`
- `affine token generate --name "token-name" --expires-at 2025-12-31`
- `affine token revoke --id ID`

### Blobs
- `affine blob delete -w ID --key KEY --permanently`
- `affine blob cleanup -w ID`

### History
- `affine history list -w ID --doc-id DOC --take 10`

### Notifications
- `affine notification list --first 20`
- `affine notification read-all`

### User
- `affine user me`
- `affine user update-profile --name "Name"`
- `affine user sign-in --email x --password y`
- `affine user sign-in --oidc` (Linux)

### Auth
- `affine auth status`

## Error Format
Errors are JSON when piped (non-TTY):
```json
{"error": "message", "code": "ERROR_CODE"}
```
Plain text when interactive:
```
Error: message
```

## ID Formats
- Workspace ID: UUID (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`)
- Document ID: UUID or short alphanumeric (`yco8IHar80`)

## Configuration
Config file: `~/.config/affine-cli/config` (key=value format; the legacy
`~/.config/affine-mcp/config` path remains a fallback)
Env vars take precedence over config file.
