# affine-cli

A standalone CLI for [AFFiNE](https://affine.pro) — manage workspaces, documents, databases, tables, comments, tags, and more from your terminal.

Built for **self-hosted AFFiNE** instances. Works with both email/password authentication and API tokens.

## Features

- **Document operations** — create, read, export, append, replace via Markdown
- **Rich text support** — bold, italic, code, strikethrough, links rendered natively in AFFiNE
- **Markdown tables** — `| col | col |` syntax converted to native `affine:table` blocks
- **Database blocks** — create databases with typed columns and rows
- **Tags, comments, blobs, notifications** — full CRUD
- **Structured JSON output** — pipe-friendly, with `--fields` filtering
- **Dry-run mode** — preview destructive operations before executing
- **AI-agent friendly** — `affine schema` outputs full JSON Schema for all commands

## Installation

### Install script

```bash
curl -sSL https://raw.githubusercontent.com/tomohiro-owada/affine-cli/main/install.sh | bash
```

Or download manually:

```bash
curl -sSL https://raw.githubusercontent.com/tomohiro-owada/affine-cli/main/install.sh -o install.sh
less install.sh
bash install.sh
rm install.sh
```

Or manually download from [GitHub Releases](https://github.com/tomohiro-owada/affine-cli/releases).

### From source

```bash
go install github.com/tomohiro-owada/affine-cli@latest
```

### Build locally

```bash
git clone https://github.com/tomohiro-owada/affine-cli.git
cd affine-cli
go build -o affine .
```

## Configuration

Set credentials via environment variables:

```bash
export AFFINE_BASE_URL=https://your-affine-instance.com
export AFFINE_EMAIL=you@example.com
export AFFINE_PASSWORD=your-password
export AFFINE_WORKSPACE_ID=your-workspace-uuid
```

Or use an API token:

```bash
export AFFINE_BASE_URL=https://your-affine-instance.com
export AFFINE_API_TOKEN=your-token
export AFFINE_WORKSPACE_ID=your-workspace-uuid
```

Config file is also supported at `~/.config/affine-mcp/config` (key=value format). Environment variables take precedence.

## Quick Start

```bash
# Check authentication
affine auth status

# List workspaces
affine workspace list

# List documents
affine doc list --fields id,title

# Create a document from Markdown
affine doc create-from-markdown --title "Hello" --content "# Hello World

This is a **bold** and *italic* text.

| Name | Score |
|------|-------|
| Alice | 100 |
| Bob | 95 |
"

# Read a document
affine doc read --doc-id <DOC_ID>

# Export as Markdown
affine doc export-markdown --doc-id <DOC_ID>
```

## Commands

### Workspaces

```bash
affine workspace list                             # List all workspaces
affine workspace get                              # Get workspace details
affine workspace create --init <file>             # Create workspace
affine workspace update --public --enable-ai      # Update settings
affine workspace delete                           # Delete workspace
```

### Documents

```bash
affine doc list [--first N] [--offset N]          # List documents
affine doc get --doc-id ID                        # Get document metadata
affine doc create --title "Title"                 # Create empty document
affine doc create-from-markdown --title T --content M  # Create from Markdown
affine doc read --doc-id ID                       # Read document blocks
affine doc export-markdown --doc-id ID            # Export as Markdown
affine doc append-paragraph --doc-id ID --text T  # Append text
affine doc append-markdown --doc-id ID --content M     # Append Markdown
affine doc replace-markdown --doc-id ID --content M    # Replace with Markdown
affine doc delete --doc-id ID                     # Delete document
affine doc publish --doc-id ID [--mode Page|Edgeless]  # Publish
affine doc revoke --doc-id ID                     # Revoke publication
```

### Database Blocks

```bash
affine db create --doc-id ID --title T --columns "Name,Status"  # Create database
affine db add-column --doc-id ID --db-block-id B --name N       # Add column
affine db add-row --doc-id ID --db-block-id B --cells '{"col-id":"value"}'  # Add row
```

### Table Blocks

```bash
affine table add-row --doc-id ID --block-id B --cells "val1,val2"  # Add row to table
```

Tables are also created automatically from Markdown table syntax in `create-from-markdown` and `append-markdown`.

### Tags

```bash
affine tag list                                   # List all tags
affine tag create --name "Tag" --color blue        # Create tag
affine tag add --doc-id ID --tag-id T              # Add tag to document
affine tag remove --doc-id ID --tag-id T           # Remove tag
affine tag list-docs --tag-id T                    # List docs with tag
```

### Comments

```bash
affine comment list --doc-id ID                   # List comments
affine comment create --doc-id ID --content "text" # Create comment
affine comment update --id ID --content "new"      # Update comment
affine comment delete --id ID                      # Delete comment
affine comment resolve --id ID --resolved          # Resolve comment
```

### User & Auth

```bash
affine auth status                                # Check auth status
affine auth sign-in --email E --password P         # Sign in
affine user me                                    # Current user info
affine user update-profile --name "Name"           # Update profile
affine user update-settings --receive-comment-notification  # Update settings
```

### Notifications

```bash
affine notification list [--first N]              # List notifications
affine notification read-all                       # Mark all as read
```

### Access Tokens

```bash
affine token list                                 # List tokens
affine token generate --name "name"                # Generate token
affine token revoke --id ID                        # Revoke token
```

### Blobs (File Storage)

```bash
affine blob upload --file path/to/file             # Upload file
affine blob delete --key KEY [--permanently]        # Delete blob
affine blob cleanup                                # Release deleted blobs
```

### History

```bash
affine history list --doc-id ID [--take N]         # List doc history
```

## Global Flags

| Flag | Description |
|------|-------------|
| `-w, --workspace` | Workspace ID (overrides `AFFINE_WORKSPACE_ID`) |
| `--dry-run` | Preview destructive operations without executing |
| `--fields` | Filter output fields (e.g., `--fields id,title`) |
| `--json` | Read structured JSON input from stdin |

## Architecture

```
affine-cli/
├── cmd/           # Cobra command definitions
├── internal/
│   ├── auth/      # Email/password sign-in
│   ├── config/    # Configuration loading (env, file)
│   ├── docops/    # Document operations (Y.js CRDT via Socket.io)
│   ├── graphql/   # GraphQL client and queries
│   ├── output/    # JSON output formatting
│   ├── socketio/  # Socket.io v4 client
│   ├── validate/  # Input validation
│   └── yjs/       # Y.js engine (goja runtime with embedded yjs bundle)
├── AGENTS.md      # AI agent integration guide
├── main.go
└── README.md
```

### How It Works

- **Metadata operations** (list, get, publish, comments, etc.) use **GraphQL** against the AFFiNE server API
- **Document content operations** (create, read, append, replace) use **Socket.io** to connect to the real-time sync protocol and manipulate **Y.js CRDT** documents directly
- **Y.js engine** runs in a [goja](https://github.com/dop251/goja) JavaScript runtime with an embedded Y.js bundle, enabling Go code to create and modify CRDT documents
- **Rich text** is stored as `Y.Text` with attributes (`bold`, `italic`, `code`, `strike`, `link`) matching AFFiNE/BlockSuite format
- **Tables** use AFFiNE's flat-key format (`prop:rows.{id}.order`, `prop:cells.{rowId}:{colId}.text`) with `Y.Text` cell values

## AI Agent Integration

This CLI is designed to be used by AI agents. Run `affine schema` to get a machine-readable JSON Schema of all commands and their parameters.

See [AGENTS.md](AGENTS.md) for the full AI agent integration guide.

## Testing

Unit tests:

```bash
go test ./...
```

Integration tests (requires a running AFFiNE instance):

```bash
export AFFINE_BASE_URL=https://your-instance.com
export AFFINE_EMAIL=test@example.com
export AFFINE_PASSWORD=password
export AFFINE_WORKSPACE_ID=your-workspace-id
go test -tags=integration -v .
```

## License

[MIT](LICENSE)
