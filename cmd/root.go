package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/affine-cli/internal/auth"
	"github.com/tomohiro-owada/affine-cli/internal/config"
	"github.com/tomohiro-owada/affine-cli/internal/graphql"
	"github.com/tomohiro-owada/affine-cli/internal/validate"
)

var (
	cfg *config.Config
	gql *graphql.Client
)

var rootCmd = &cobra.Command{
	Use:   "affine",
	Short: "CLI for AFFiNE workspace management",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Allow schema/auth commands to skip config
		if cmd.Name() == "schema" || cmd.Name() == "version" {
			return nil
		}

		cfg = config.Load()

		if cfg.APIToken == "" && cfg.Cookie == "" && cmd != userSignInCmd && cmd != userOAuthCallbackCmd {
			token, err := auth.SessionAccessToken(context.Background(), cfg.BaseURL, cfg.WSClientVersion)
			if err != nil {
				return fmt.Errorf("OIDC session: %w", err)
			}
			cfg.APIToken = token
			cfg.OIDCSession = token != ""
		}

		// Override workspace ID from flag if provided
		if ws, _ := cmd.Flags().GetString("workspace"); ws != "" {
			cfg.DefaultWorkspaceID = ws
		}

		// Read JSON input from stdin if --json flag is set
		if jsonInput, _ := cmd.Flags().GetBool("json"); jsonInput {
			if err := readJSONInput(cmd); err != nil {
				return fmt.Errorf("JSON input error: %w", err)
			}
		}

		var extraHeaders map[string]string
		if cfg.HeadersJSON != "" {
			_ = json.Unmarshal([]byte(cfg.HeadersJSON), &extraHeaders)
		}

		gql = graphql.NewClient(
			cfg.GraphQLEndpoint(),
			cfg.APIToken,
			cfg.Cookie,
			extraHeaders,
		)

		// Auto sign-in if no token/cookie but email+password are available
		if cfg.APIToken == "" && cfg.Cookie == "" && cfg.Email != "" && cfg.Password != "" {
			cookie, err := auth.SignIn(context.Background(), cfg.BaseURL, cfg.Email, cfg.Password)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto sign-in failed: %v\n", err)
			} else {
				gql.SetCookie(cookie)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringP("workspace", "w", "", "Workspace ID (overrides AFFINE_WORKSPACE_ID)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview destructive operations without executing")
	rootCmd.PersistentFlags().StringSlice("fields", nil, "Filter output fields (e.g., --fields id,title)")
	rootCmd.PersistentFlags().Bool("json", false, "Read structured JSON input from stdin")
}

func Execute() error {
	return rootCmd.Execute()
}

// requireWorkspace returns the workspace ID or an error if not set.
// Also validates format.
func requireWorkspace() (string, error) {
	if cfg.DefaultWorkspaceID == "" {
		return "", fmt.Errorf("workspace ID required: use -w flag or set AFFINE_WORKSPACE_ID")
	}
	if err := validate.WorkspaceID(cfg.DefaultWorkspaceID); err != nil {
		return "", err
	}
	return cfg.DefaultWorkspaceID, nil
}

func ctx() context.Context {
	return context.Background()
}

// isDryRun checks if --dry-run flag is set.
func isDryRun(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("dry-run")
	return v
}

// getFields returns the --fields filter list.
func getFields(cmd *cobra.Command) []string {
	fields, _ := cmd.Flags().GetStringSlice("fields")
	if len(fields) == 1 && strings.Contains(fields[0], ",") {
		fields = strings.Split(fields[0], ",")
	}
	return fields
}

// readJSONInput reads JSON from stdin and sets flag values from it.
func readJSONInput(cmd *cobra.Command) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parse JSON input: %w", err)
	}

	// Set flag values from JSON keys
	for key, val := range input {
		flag := cmd.Flags().Lookup(key)
		if flag == nil {
			// Try kebab-case conversion
			kebab := strings.ReplaceAll(key, "_", "-")
			flag = cmd.Flags().Lookup(kebab)
		}
		if flag == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			_ = cmd.Flags().Set(flag.Name, v)
		case float64:
			_ = cmd.Flags().Set(flag.Name, fmt.Sprintf("%v", v))
		case bool:
			_ = cmd.Flags().Set(flag.Name, fmt.Sprintf("%v", v))
		}
	}
	return nil
}
