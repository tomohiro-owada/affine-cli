package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/affine-cli/internal/docops"
	"github.com/tomohiro-owada/affine-cli/internal/graphql"
	"github.com/tomohiro-owada/affine-cli/internal/output"
	"github.com/tomohiro-owada/affine-cli/internal/validate"
	"github.com/tomohiro-owada/affine-cli/internal/workspacetitles"
)

func init() {
	rootCmd.AddCommand(docCmd)
	docCmd.AddCommand(docListCmd)
	docCmd.AddCommand(docGetCmd)
	docCmd.AddCommand(docPublishCmd)
	docCmd.AddCommand(docRevokeCmd)
	docCmd.AddCommand(docCreateCmd)
	docCmd.AddCommand(docCreateFromMarkdownCmd)
	docCmd.AddCommand(docReadCmd)
	docCmd.AddCommand(docDeleteCmd)
	docCmd.AddCommand(docExportMarkdownCmd)
	docCmd.AddCommand(docAppendParagraphCmd)
	docCmd.AddCommand(docAppendMarkdownCmd)
	docCmd.AddCommand(docReplaceMarkdownCmd)

	docListCmd.Flags().Int("first", 20, "Number of docs to return")
	docListCmd.Flags().Int("offset", 0, "Offset for pagination")
	docListCmd.Flags().String("after", "", "Cursor for pagination")
	docListCmd.Flags().Bool("skip-yjs-titles", false, "Skip merging display titles from workspace Yjs (GraphQL only)")

	docGetCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docGetCmd.MarkFlagRequired("doc-id")

	docPublishCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docPublishCmd.MarkFlagRequired("doc-id")
	docPublishCmd.Flags().String("mode", "Page", "Publish mode (Page or Edgeless)")

	docRevokeCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docRevokeCmd.MarkFlagRequired("doc-id")

	docCreateCmd.Flags().String("title", "", "Document title")

	docCreateFromMarkdownCmd.Flags().String("title", "", "Document title")
	docCreateFromMarkdownCmd.Flags().String("content", "", "Markdown content (or use stdin)")

	docReadCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docReadCmd.MarkFlagRequired("doc-id")

	docDeleteCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docDeleteCmd.MarkFlagRequired("doc-id")

	docExportMarkdownCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docExportMarkdownCmd.MarkFlagRequired("doc-id")

	docAppendParagraphCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docAppendParagraphCmd.MarkFlagRequired("doc-id")
	docAppendParagraphCmd.Flags().String("text", "", "Paragraph text (required)")
	_ = docAppendParagraphCmd.MarkFlagRequired("text")
	docAppendParagraphCmd.Flags().String("type", "text", "Paragraph type (text, h1, h2, h3, quote)")

	docAppendMarkdownCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docAppendMarkdownCmd.MarkFlagRequired("doc-id")
	docAppendMarkdownCmd.Flags().String("content", "", "Markdown content (or use stdin)")

	docReplaceMarkdownCmd.Flags().String("doc-id", "", "Document ID (required)")
	_ = docReplaceMarkdownCmd.MarkFlagRequired("doc-id")
	docReplaceMarkdownCmd.Flags().String("content", "", "Markdown content (or use stdin)")
}

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Manage documents",
}

// --- GraphQL commands ---

var docListCmd = &cobra.Command{
	Use:   "list",
	Short: "List documents in a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		first, _ := cmd.Flags().GetInt("first")
		offset, _ := cmd.Flags().GetInt("offset")
		after, _ := cmd.Flags().GetString("after")
		vars := map[string]any{
			"workspaceId": ws, "first": first, "offset": offset,
		}
		if after != "" {
			vars["after"] = after
		}
		data, err := gql.Request(ctx(), graphql.ListDocsQuery, vars)
		if err != nil {
			return err
		}
		skipYjs, _ := cmd.Flags().GetBool("skip-yjs-titles")
		if !skipYjs {
			if titles, err := workspacetitles.FetchPageTitles(ctx(), cfg.BaseURL, ws, gql.Cookie(), gql.Bearer(), gql.ExtraHeaders()); err == nil && len(titles) > 0 {
				data = workspacetitles.MergeDocListTitles(data, titles)
			} else if err != nil {
				output.WarnWithCode("YJS_TITLE_ENRICHMENT_FAILED", "doc list title enrichment (Yjs): %v", err)
			}
		}
		if fields := getFields(cmd); len(fields) > 0 {
			output.FilteredJSON(data, fields)
			return nil
		}
		output.RawJSON(data)
		return nil
	},
}

var docGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get document metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		data, err := gql.Request(ctx(), graphql.GetDocQuery, map[string]any{
			"workspaceId": ws, "docId": docID,
		})
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

var docPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		mode, _ := cmd.Flags().GetString("mode")
		if isDryRun(cmd) {
			output.DryRun("publish document", map[string]any{
				"workspace_id": ws, "doc_id": docID, "mode": mode,
			})
			return nil
		}
		data, err := gql.Request(ctx(), graphql.PublishDocMutation, map[string]any{
			"workspaceId": ws, "docId": docID, "mode": mode,
		})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

var docRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke public access to a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := requireWorkspace()
		if err != nil {
			return err
		}
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		if isDryRun(cmd) {
			output.DryRun("revoke document publication", map[string]any{
				"workspace_id": ws, "doc_id": docID,
			})
			return nil
		}
		data, err := gql.Request(ctx(), graphql.RevokeDocMutation, map[string]any{
			"workspaceId": ws, "docId": docID,
		})
		if err != nil {
			return err
		}
		output.RawJSON(data)
		return nil
	},
}

// --- Socket.io + Y.js commands ---

func connectDocOps(cmd *cobra.Command) (*docops.Session, error) {
	ws, err := requireWorkspace()
	if err != nil {
		return nil, err
	}
	return docops.Connect(cfg, ws)
}

var docCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new empty document",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		if isDryRun(cmd) {
			output.DryRun("create document", map[string]any{"title": title})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		docID, err := sess.CreateDoc(title)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{
			"doc_id":       docID,
			"workspace_id": sess.WorkspaceID,
			"title":        title,
		})
		return nil
	},
}

var docCreateFromMarkdownCmd = &cobra.Command{
	Use:   "create-from-markdown",
	Short: "Create a new document from markdown content",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			content = readStdin()
		}
		if content == "" {
			return fmt.Errorf("content required: use --content or pipe via stdin")
		}
		if isDryRun(cmd) {
			output.DryRun("create document from markdown", map[string]any{
				"title":          title,
				"content_length": len(content),
			})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		docID, err := sess.CreateDocFromMarkdown(title, content)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{
			"doc_id":       docID,
			"workspace_id": sess.WorkspaceID,
			"title":        title,
		})
		return nil
	},
}

var docReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read document content (blocks and plain text)",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		blocks, plainText, err := sess.ReadDoc(docID)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{
			"doc_id":     docID,
			"blocks":     blocks,
			"plain_text": plainText,
			"block_count": len(blocks),
		})
		return nil
	},
}

var docDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		if isDryRun(cmd) {
			ws, _ := requireWorkspace()
			output.DryRun("delete document", map[string]any{
				"workspace_id": ws, "doc_id": docID,
			})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		err = sess.DeleteDoc(docID)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{"deleted": true, "doc_id": docID})
		return nil
	},
}

var docExportMarkdownCmd = &cobra.Command{
	Use:   "export-markdown",
	Short: "Export document as markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		md, err := sess.ExportMarkdown(docID)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{
			"doc_id":   docID,
			"markdown": md,
		})
		return nil
	},
}

var docAppendParagraphCmd = &cobra.Command{
	Use:   "append-paragraph",
	Short: "Append a paragraph to a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		text, _ := cmd.Flags().GetString("text")
		ptype, _ := cmd.Flags().GetString("type")
		if isDryRun(cmd) {
			output.DryRun("append paragraph", map[string]any{
				"doc_id": docID, "text": text, "type": ptype,
			})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		err = sess.AppendParagraph(docID, text, ptype)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{"appended": true, "doc_id": docID})
		return nil
	},
}

var docAppendMarkdownCmd = &cobra.Command{
	Use:   "append-markdown",
	Short: "Append markdown content to a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			content = readStdin()
		}
		if content == "" {
			return fmt.Errorf("content required: use --content or pipe via stdin")
		}
		if isDryRun(cmd) {
			output.DryRun("append markdown", map[string]any{
				"doc_id": docID, "content_length": len(content),
			})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		count, err := sess.AppendMarkdown(docID, content)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{"appended": true, "doc_id": docID, "block_count": count})
		return nil
	},
}

var docReplaceMarkdownCmd = &cobra.Command{
	Use:   "replace-markdown",
	Short: "Replace document content with markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		docID, _ := cmd.Flags().GetString("doc-id")
		if err := validate.DocID(docID); err != nil {
			return err
		}
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			content = readStdin()
		}
		if content == "" {
			return fmt.Errorf("content required: use --content or pipe via stdin")
		}
		if isDryRun(cmd) {
			output.DryRun("replace document with markdown", map[string]any{
				"doc_id": docID, "content_length": len(content),
			})
			return nil
		}
		sess, err := connectDocOps(cmd)
		if err != nil {
			return err
		}
		defer sess.Close()
		count, err := sess.ReplaceWithMarkdown(docID, content)
		if err != nil {
			return err
		}
		output.JSON(map[string]any{"replaced": true, "doc_id": docID, "block_count": count})
		return nil
	},
}

func readStdin() string {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "" // interactive terminal, no piped input
	}
	data, err := os.ReadFile("/dev/stdin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
