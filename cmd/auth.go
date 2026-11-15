package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/affine-cli/internal/graphql"
	"github.com/tomohiro-owada/affine-cli/internal/output"
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authStatusCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication management",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		status := map[string]any{
			"authenticated": false,
			"method":        "none",
		}

		if cfg.OIDCSession {
			status["method"] = "oidc"
		} else if cfg.APIToken != "" {
			status["method"] = "api_token"
		} else if cfg.Cookie != "" {
			status["method"] = "cookie"
		} else if cfg.Email != "" {
			status["method"] = "auto_signin"
		}

		// Try to fetch current user to verify auth works
		data, err := gql.Request(ctx(), graphql.CurrentUserQuery, nil)
		if err != nil {
			status["error"] = err.Error()
			output.JSON(status)
			return fmt.Errorf("authentication check failed: %w", err)
		}

		status["authenticated"] = true
		status["user"] = data

		if cfg.DefaultWorkspaceID != "" {
			status["workspace_id"] = cfg.DefaultWorkspaceID
		}

		output.JSON(status)
		return nil
	},
}
