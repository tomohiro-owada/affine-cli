package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/affine-cli/internal/auth"
	"github.com/tomohiro-owada/affine-cli/internal/graphql"
	"github.com/tomohiro-owada/affine-cli/internal/output"
	"github.com/tomohiro-owada/affine-cli/internal/validate"
)

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userMeCmd)
	userCmd.AddCommand(userUpdateProfileCmd)
	userCmd.AddCommand(userUpdateSettingsCmd)
	userCmd.AddCommand(userSignInCmd)
	userCmd.AddCommand(userOAuthCallbackCmd)

	userUpdateProfileCmd.Flags().String("name", "", "Display name")
	userUpdateProfileCmd.Flags().String("avatar-url", "", "Avatar URL")

	userUpdateSettingsCmd.Flags().Bool("receive-comment-notification", false, "Receive comment notifications")
	userUpdateSettingsCmd.Flags().Bool("receive-mention-notification", false, "Receive mention notifications")

	userSignInCmd.Flags().String("email", "", "Email address")
	userSignInCmd.Flags().String("password", "", "Password")
	userSignInCmd.Flags().Bool("oidc", false, "Sign in through the configured OIDC provider")
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management",
}

var userMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Get current user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := gql.Request(ctx(), graphql.CurrentUserQuery, nil)
		if err != nil {
			return err
		}
		if fields := getFields(cmd); len(fields) > 0 {
			output.FilteredJSON(data, fields)
			return nil
		}
		output.RawJSON(data)
		return nil
	},
}

var userUpdateProfileCmd = &cobra.Command{
	Use:   "update-profile",
	Short: "Update user profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		input := map[string]any{}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			if err := validate.NoControlChars("name", v); err != nil {
				return err
			}
			input["name"] = v
		}
		if cmd.Flags().Changed("avatar-url") {
			v, _ := cmd.Flags().GetString("avatar-url")
			input["avatarUrl"] = v
		}
		if len(input) == 0 {
			return fmt.Errorf("at least one of --name or --avatar-url is required")
		}

		if isDryRun(cmd) {
			output.DryRun("update profile", input)
			return nil
		}

		data, err := gql.Request(ctx(), graphql.UpdateProfileMutation, map[string]any{"input": input})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

var userUpdateSettingsCmd = &cobra.Command{
	Use:   "update-settings",
	Short: "Update notification preferences",
	RunE: func(cmd *cobra.Command, args []string) error {
		input := map[string]any{}
		if cmd.Flags().Changed("receive-comment-notification") {
			v, _ := cmd.Flags().GetBool("receive-comment-notification")
			input["receiveCommentNotification"] = v
		}
		if cmd.Flags().Changed("receive-mention-notification") {
			v, _ := cmd.Flags().GetBool("receive-mention-notification")
			input["receiveMentionNotification"] = v
		}
		if len(input) == 0 {
			return fmt.Errorf("at least one setting flag is required")
		}
		if isDryRun(cmd) {
			output.DryRun("update settings", input)
			return nil
		}
		data, err := gql.Request(ctx(), graphql.UpdateSettingsMutation, map[string]any{"input": input})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

var userSignInCmd = &cobra.Command{
	Use:   "sign-in",
	Short: "Sign in with email/password or OIDC",
	RunE: func(cmd *cobra.Command, args []string) error {
		oidc, _ := cmd.Flags().GetBool("oidc")
		if oidc {
			if err := auth.SignInOIDC(ctx(), cfg.BaseURL, cfg.WSClientVersion); err != nil {
				return err
			}
			output.JSON(map[string]any{
				"status": "authenticated",
				"method": "oidc",
			})
			return nil
		}
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		if email == "" {
			email = cfg.Email
		}
		if password == "" {
			password = cfg.Password
		}
		if email == "" || password == "" {
			return fmt.Errorf("email and password required (use flags or env vars)")
		}
		cookie, err := auth.SignIn(ctx(), cfg.BaseURL, email, password)
		if err != nil {
			return err
		}
		gql.SetCookie(cookie)
		output.JSON(map[string]any{
			"status": "authenticated",
			"cookie": cookie,
		})
		return nil
	},
}

var userOAuthCallbackCmd = &cobra.Command{
	Use:    "oauth-callback URL",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return auth.DeliverOIDCCallback(args[0])
	},
}
