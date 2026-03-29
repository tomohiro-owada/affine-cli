package cmd

import (
	"bytes"
	"fmt"
	"mime/multipart"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/affine-cli/internal/graphql"
	"github.com/tomohiro-owada/affine-cli/internal/output"
	"github.com/tomohiro-owada/affine-cli/internal/workspacetitles"
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceGetCmd)
	workspaceCmd.AddCommand(workspaceCreateCmd)
	workspaceCmd.AddCommand(workspaceUpdateCmd)
	workspaceCmd.AddCommand(workspaceDeleteCmd)
	workspaceCmd.AddCommand(workspaceListPagesCmd)

	workspaceCreateCmd.Flags().String("name", "Untitled", "Workspace name")

	workspaceUpdateCmd.Flags().Bool("public", false, "Make workspace public")
	workspaceUpdateCmd.Flags().Bool("enable-ai", false, "Enable AI features")
}

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := gql.Request(ctx(), graphql.ListWorkspacesQuery, nil)
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

var workspaceListPagesCmd = &cobra.Command{
	Use:   "list-pages",
	Short: "List pages (id, title, parentId) from workspace Yjs index for folder navigation",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		raw, err := workspacetitles.FetchPageList(ctx(), cfg.BaseURL, ws, gql.Cookie(), gql.Bearer(), gql.ExtraHeaders())
		if err != nil {
			return err
		}
		output.RawJSON(raw)
		return nil
	},
}

var workspaceGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get workspace details",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		data, err := gql.Request(ctx(), graphql.GetWorkspaceQuery, map[string]any{"id": ws})
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

var workspaceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun(cmd) {
			name, _ := cmd.Flags().GetString("name")
			output.DryRun("create workspace", map[string]any{"name": name})
			return nil
		}

		// AFFiNE requires an "init" Upload for createWorkspace
		// We send an empty Y.Doc as the init blob
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		ops := fmt.Sprintf(`{"query":%q,"variables":{"init":null}}`, graphql.CreateWorkspaceMutation)
		_ = writer.WriteField("operations", ops)
		_ = writer.WriteField("map", `{"0":["variables.init"]}`)

		// Empty init blob
		part, err := writer.CreateFormFile("0", "init.bin")
		if err != nil {
			return err
		}
		part.Write([]byte{}) // empty
		writer.Close()

		data, err := gql.RequestMultipart(ctx(), &buf, writer.FormDataContentType())
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

var workspaceUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update workspace settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		input := map[string]any{"id": ws}
		if cmd.Flags().Changed("public") {
			v, _ := cmd.Flags().GetBool("public")
			input["public"] = v
		}
		if cmd.Flags().Changed("enable-ai") {
			v, _ := cmd.Flags().GetBool("enable-ai")
			input["enableAi"] = v
		}
		if isDryRun(cmd) {
			output.DryRun("update workspace", map[string]any{
				"workspace_id": ws, "changes": input,
			})
			return nil
		}
		data, err := gql.Request(ctx(), graphql.UpdateWorkspaceMutation, map[string]any{"input": input})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		if isDryRun(cmd) {
			output.DryRun("delete workspace", map[string]any{"workspace_id": ws})
			return nil
		}
		data, err := gql.Request(ctx(), graphql.DeleteWorkspaceMutation, map[string]any{"id": ws})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}
