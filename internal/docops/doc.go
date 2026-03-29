package docops

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tomohiro-owada/affine-cli/internal/yjs"
)

// GenerateDocID creates a random short doc ID (10 chars, base62-ish).
func GenerateDocID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)[:10]
}

// blockChildIDs returns ordered child block IDs from sys:children (Y.Array as []any).
func blockChildIDs(b map[string]any) []string {
	raw, ok := b["sys:children"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		switch v := x.(type) {
		case string:
			out = append(out, v)
		case float64:
			out = append(out, strconv.FormatInt(int64(v), 10))
		}
	}
	return out
}

// plainTextFromProp reads a string or { delta: [...] } snapshot from a block map key.
func plainTextFromProp(b map[string]any, key string) string {
	if s, ok := b[key].(string); ok {
		return s
	}
	m, ok := b[key].(map[string]any)
	if !ok {
		return ""
	}
	delta, ok := m["delta"].([]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, op := range delta {
		om, ok := op.(map[string]any)
		if !ok {
			continue
		}
		if ins, ok := om["insert"].(string); ok {
			sb.WriteString(ins)
		}
	}
	return sb.String()
}

// blockPropPlainText returns prop:text as a string, including structured { delta: [...] } snapshots.
func blockPropPlainText(b map[string]any) string {
	return plainTextFromProp(b, "prop:text")
}

func typeFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return ""
		}
		if x == "$blocksuite:internal:native$" {
			return ""
		}
		return normalizeBlockType(x)
	case float64:
		n := int(x)
		if n >= 1 && n <= 6 && float64(n) == x {
			return "h" + strconv.Itoa(n)
		}
	case map[string]any:
		// BlockSuite boxed natives: { type: "$blocksuite:internal:native$", value: "h1" }
		if tag, ok := x["type"].(string); ok && tag == "$blocksuite:internal:native$" {
			if t := typeFromAny(x["value"]); t != "" {
				return t
			}
		}
		for _, key := range []string{"value", "type", "kind"} {
			if t := typeFromAny(x[key]); t != "" {
				return t
			}
		}
	}
	return ""
}

// blockContentType returns prop:type / sys:type for paragraph and list blocks in a form
// our markdown export understands (handles case variants, numeric heading levels, aliases).
func blockContentType(b map[string]any) string {
	var candidates []string
	add := func(t string) {
		if t == "" {
			return
		}
		for _, x := range candidates {
			if x == t {
				return
			}
		}
		candidates = append(candidates, t)
	}
	for _, key := range []string{"prop:type", "prop:type.value", "sys:type", "type"} {
		add(typeFromAny(b[key]))
	}
	for k, v := range b {
		if k == "prop:type" || k == "prop:type.value" {
			continue
		}
		if strings.HasPrefix(k, "prop:type") {
			add(typeFromAny(v))
		}
	}
	for _, t := range candidates {
		if len(t) == 2 && t[0] == 'h' && t[1] >= '1' && t[1] <= '6' {
			return t
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func normalizeBlockType(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "h1", "h2", "h3", "h4", "h5", "h6",
		"text", "quote",
		"bulleted", "numbered", "todo", "toggle":
		return s
	}
	if strings.HasPrefix(s, "heading") {
		rest := strings.TrimPrefix(s, "heading")
		if rest == "" {
			return "h1"
		}
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 6 {
			return "h" + strconv.Itoa(n)
		}
	}
	switch s {
	case "subtitle", "subheading":
		return "h2"
	case "title":
		return "h1"
	}
	return s
}

func pageMarkdownPrefix(blocks map[string]map[string]any) string {
	t := pageTitleFromBlocks(blocks)
	if t == "" {
		return ""
	}
	return "# " + t + "\n\n"
}

// pageTitleFromBlocks returns the affine:page document title as plain text.
func pageTitleFromBlocks(blocks map[string]map[string]any) string {
	for _, b := range blocks {
		flavour, _ := b["sys:flavour"].(string)
		if flavour != "affine:page" {
			continue
		}
		t := strings.TrimSpace(plainTextFromProp(b, "prop:title"))
		if t != "" {
			return t
		}
	}
	return ""
}

// stripMirrorOfExportedDocTitle removes a leading "# <title>" line (and following blank lines)
// when it matches the page title. ExportMarkdown prepends that line via pageMarkdownPrefix; without
// stripping, replace-markdown would create a duplicate h1 in the note body.
func stripMirrorOfExportedDocTitle(md string, docTitle string) string {
	docTitle = strings.TrimSpace(docTitle)
	if docTitle == "" {
		return md
	}
	lines := strings.Split(md, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return md
	}
	trimmed := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(trimmed, "#") {
		return md
	}
	level := 0
	for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
		level++
	}
	if level != 1 {
		return md
	}
	headingText := strings.TrimSpace(trimmed[level:])
	if !strings.EqualFold(headingText, docTitle) {
		return md
	}
	i++
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return strings.Join(lines[i:], "\n")
}

// CreateDoc creates a new empty document in the workspace.
// It updates the workspace root doc's meta.pages and creates the doc's Y.Doc.
func (s *Session) CreateDoc(title string) (string, error) {
	docID := GenerateDocID()

	// 1. Update workspace root doc to register the new doc
	rootEngID, err := s.LoadWorkspaceRoot()
	if err != nil {
		return "", fmt.Errorf("load workspace root: %w", err)
	}

	err = s.PushDocDelta(rootEngID, s.WorkspaceID, func() error {
		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var meta = doc.getMap("meta");
				var pages = meta.get("pages");
				if (!pages) {
					pages = new Y.Array();
					meta.set("pages", pages);
				}
				var pageMeta = new Y.Map();
				pageMeta.set("id", %q);
				var titleText = new Y.Text();
				titleText.insert(0, %q, {});
				pageMeta.set("title", titleText);
				pageMeta.set("createDate", Date.now());
				pages.push([pageMeta]);
				return "ok";
			})()
		`, rootEngID, docID, title)
		_, err := s.Engine.RunScript(script)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("register doc in workspace: %w", err)
	}

	// 2. Create the doc's own Y.Doc with basic structure
	newEngID, err := s.Engine.NewDoc()
	if err != nil {
		return "", err
	}

	script := fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");

			var page = new Y.Map();
			page.set("sys:id", "page-root");
			page.set("sys:flavour", "affine:page");
			page.set("sys:version", 2);
			var titleText = new Y.Text();
			titleText.insert(0, %q, {});
			page.set("prop:title", titleText);
			var pageChildren = new Y.Array();
			pageChildren.push(["surface-1", "note-1"]);
			page.set("sys:children", pageChildren);
			blocks.set("page-root", page);

			var surface = new Y.Map();
			surface.set("sys:id", "surface-1");
			surface.set("sys:flavour", "affine:surface");
			surface.set("sys:version", 5);
			surface.set("sys:children", new Y.Array());
			blocks.set("surface-1", surface);

			var note = new Y.Map();
			note.set("sys:id", "note-1");
			note.set("sys:flavour", "affine:note");
			note.set("sys:version", 1);
			note.set("sys:children", new Y.Array());
			blocks.set("note-1", note);

			return "ok";
		})()
	`, newEngID, title)
	_, err = s.Engine.RunScript(script)
	if err != nil {
		return "", fmt.Errorf("create doc structure: %w", err)
	}

	b64, err := s.Engine.EncodeStateAsUpdate(newEngID)
	if err != nil {
		return "", fmt.Errorf("encode doc: %w", err)
	}

	err = s.Client.PushDocUpdate(s.WorkspaceID, docID, b64)
	if err != nil {
		return "", fmt.Errorf("push doc: %w", err)
	}

	s.Engine.FreeDoc(newEngID)
	return docID, nil
}

// ReadDoc reads a document's blocks and returns structured data.
type BlockInfo struct {
	ID       string `json:"id"`
	Flavour  string `json:"flavour"`
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	Language string `json:"language,omitempty"`
}

func (s *Session) ReadDoc(docID string) ([]BlockInfo, string, error) {
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return nil, "", err
	}
	defer s.Engine.FreeDoc(engDocID)

	blocks, err := s.Engine.ReadBlocks(engDocID)
	if err != nil {
		return nil, "", err
	}

	var result []BlockInfo

	for id, b := range blocks {
		flavour, _ := b["sys:flavour"].(string)
		if flavour == "affine:page" || flavour == "affine:surface" || flavour == "affine:note" {
			continue
		}

		btype := blockContentType(b)
		text := blockPropPlainText(b)
		lang, _ := b["prop:language"].(string)

		liveType := s.Engine.BlockPropTypeString(engDocID, id)
		if liveType != "" {
			btype = normalizeBlockType(liveType)
		}
		liveText := s.Engine.BlockPropTextString(engDocID, id)
		if liveText != "" {
			text = liveText
		}

		info := BlockInfo{
			ID:       id,
			Flavour:  flavour,
			Type:     btype,
			Text:     text,
			Language: lang,
		}
		result = append(result, info)
	}

	order, ordErr := s.Engine.NoteChildOrder(engDocID)
	if ordErr == nil && len(order) > 0 {
		idx := make(map[string]int, len(order))
		for i, bid := range order {
			idx[bid] = i
		}
		sort.SliceStable(result, func(i, j int) bool {
			ii, okI := idx[result[i].ID]
			ij, okJ := idx[result[j].ID]
			switch {
			case okI && okJ:
				return ii < ij
			case okI:
				return true
			case okJ:
				return false
			default:
				return result[i].ID < result[j].ID
			}
		})
	} else {
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].ID < result[j].ID
		})
	}

	var textParts []string
	for _, info := range result {
		if info.Text != "" {
			textParts = append(textParts, info.Text)
		}
	}
	plainText := strings.Join(textParts, "\n")
	return result, plainText, nil
}

// DeleteDoc removes a document from the workspace.
func (s *Session) DeleteDoc(docID string) error {
	// Remove from workspace root doc's meta.pages
	rootEngID, err := s.LoadWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("load workspace root: %w", err)
	}

	err = s.PushDocDelta(rootEngID, s.WorkspaceID, func() error {
		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var meta = doc.getMap("meta");
				var pages = meta.get("pages");
				if (!pages) return "no pages";
				for (var i = 0; i < pages.length; i++) {
					var p = pages.get(i);
					if (p && p.get && p.get("id") === %q) {
						pages.delete(i, 1);
						return "deleted";
					}
				}
				return "not found";
			})()
		`, rootEngID, docID)
		_, err := s.Engine.RunScript(script)
		return err
	})
	if err != nil {
		return fmt.Errorf("remove doc from meta: %w", err)
	}

	// Send delete event
	s.Client.DeleteDoc(s.WorkspaceID, docID)
	return nil
}

// exportEngine reads live block props during markdown export (implemented by *yjs.Engine).
type exportEngine interface {
	BlockPropTypeString(docID int, blockID string) string
	BlockPropTextAffineMarkdown(docID int, blockID string) string
}

// buildMarkdownExport applies the same rules as ExportMarkdown to an in-memory doc.
// Used by ExportMarkdown and by tests (boxed prop:type, flat keys, etc.).
func buildMarkdownExport(e exportEngine, engDocID int, blocks map[string]map[string]any, order []string) string {
	var md strings.Builder
	if prefix := pageMarkdownPrefix(blocks); prefix != "" {
		md.WriteString(prefix)
	}

	emitBlock := func(id string) {
		b, ok := blocks[id]
		if !ok {
			return
		}
		flavour, _ := b["sys:flavour"].(string)
		btype := blockContentType(b)
		if live := e.BlockPropTypeString(engDocID, id); live != "" {
			btype = normalizeBlockType(live)
		}
		text := blockPropPlainText(b)
		if liveMd := e.BlockPropTextAffineMarkdown(engDocID, id); liveMd != "" {
			text = liveMd
		}
		lang, _ := b["prop:language"].(string)

		switch flavour {
		case "affine:paragraph":
			switch btype {
			case "h1":
				md.WriteString("# " + text + "\n\n")
			case "h2":
				md.WriteString("## " + text + "\n\n")
			case "h3":
				md.WriteString("### " + text + "\n\n")
			case "h4":
				md.WriteString("#### " + text + "\n\n")
			case "h5":
				md.WriteString("##### " + text + "\n\n")
			case "h6":
				md.WriteString("###### " + text + "\n\n")
			case "quote":
				md.WriteString("> " + text + "\n\n")
			default:
				if text != "" {
					md.WriteString(text + "\n\n")
				}
			}
		case "affine:list":
			switch btype {
			case "bulleted", "toggle":
				md.WriteString("- " + text + "\n")
			case "numbered":
				md.WriteString("1. " + text + "\n")
			case "todo":
				checked, _ := b["prop:checked"].(bool)
				if checked {
					md.WriteString("- [x] " + text + "\n")
				} else {
					md.WriteString("- [ ] " + text + "\n")
				}
			default:
				md.WriteString("- " + text + "\n")
			}
		case "affine:code":
			md.WriteString("```" + lang + "\n" + text + "\n```\n\n")
		case "affine:divider":
			md.WriteString("---\n\n")
		}
	}

	var walk func(string)
	walk = func(id string) {
		b, ok := blocks[id]
		if !ok {
			return
		}
		flavour, _ := b["sys:flavour"].(string)
		// Descend into callouts (and similar hubs) so nested headings/lists export.
		if flavour == "affine:callout" {
			for _, cid := range blockChildIDs(b) {
				walk(cid)
			}
			return
		}
		emitBlock(id)
		if flavour == "affine:list" {
			for _, cid := range blockChildIDs(b) {
				walk(cid)
			}
		}
		if flavour == "affine:paragraph" {
			for _, cid := range blockChildIDs(b) {
				walk(cid)
			}
		}
	}

	if len(order) > 0 {
		for _, id := range order {
			walk(id)
		}
	} else {
		ids := make([]string, 0, len(blocks))
		for id := range blocks {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			walk(id)
		}
	}

	return md.String()
}

// ExportMarkdown exports a document as markdown.
func (s *Session) ExportMarkdown(docID string) (string, error) {
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return "", err
	}
	defer s.Engine.FreeDoc(engDocID)

	blocks, err := s.Engine.ReadBlocks(engDocID)
	if err != nil {
		return "", err
	}

	order, err := s.Engine.NoteChildOrder(engDocID)
	if err != nil {
		return "", err
	}

	return buildMarkdownExport(s.Engine, engDocID, blocks, order), nil
}

// AppendParagraph appends a single paragraph block to a document.
func (s *Session) AppendParagraph(docID, text, paragraphType string) error {
	if paragraphType == "" {
		paragraphType = "text"
	}
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return err
	}

	blockID := "p-" + GenerateDocID()

	return s.PushDocDelta(engDocID, docID, func() error {
		err := s.Engine.CreateFormattedBlock(engDocID, blockID, "affine:paragraph", paragraphType, text)
		if err != nil {
			return err
		}
		// Add children array and add to note
		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var blocks = doc.getMap("blocks");
				var block = blocks.get(%q);
				block.set("sys:children", new Y.Array());
				// Find note block and append
				blocks.forEach(function(b, id) {
					if (!(b instanceof Y.Map)) return;
					if (b.get("sys:flavour") === "affine:note") {
						var children = b.get("sys:children");
						if (children instanceof Y.Array) {
							children.push([%q]);
						}
					}
				});
				return "ok";
			})()
		`, engDocID, blockID, blockID)
		_, err = s.Engine.RunScript(script)
		return err
	})
}

// AppendMarkdown parses markdown and appends multiple blocks.
func (s *Session) AppendMarkdown(docID, markdown string) (int, error) {
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return 0, err
	}

	lines := parseMarkdownToLines(markdown)
	if len(lines) == 0 {
		return 0, nil
	}

	count := 0
	err = s.PushDocDelta(engDocID, docID, func() error {
		var blockIDs []string
		for _, line := range lines {
			blockID := "b-" + GenerateDocID()

			// Handle table blocks (affine:table)
			if line.flavour == "affine:database" && len(line.tableHeaders) > 0 {
				if err := createTableBlock(s.Engine, engDocID, blockID, line.tableHeaders, line.tableRows); err != nil {
					return err
				}
				blockIDs = append(blockIDs, blockID)
				count++
				continue
			}

			err := s.Engine.CreateFormattedBlock(engDocID, blockID, line.flavour, line.btype, line.text)
			if err != nil {
				return err
			}
			// Set sys:children, language if code
			extra := ""
			if line.language != "" {
				extra = fmt.Sprintf(`block.set("prop:language", %q);`, line.language)
			}
			if line.checked {
				extra += `block.set("prop:checked", true);`
			}
			script := fmt.Sprintf(`
				(function() {
					var doc = globalThis._docs[%d];
					var block = doc.getMap("blocks").get(%q);
					block.set("sys:children", new Y.Array());
					%s
					return "ok";
				})()
			`, engDocID, blockID, extra)
			_, err = s.Engine.RunScript(script)
			if err != nil {
				return err
			}
			blockIDs = append(blockIDs, blockID)
			count++
		}

		// Add all block IDs to note's children
		idsJS := `[`
		for i, id := range blockIDs {
			if i > 0 {
				idsJS += ","
			}
			idsJS += fmt.Sprintf("%q", id)
		}
		idsJS += `]`

		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var blocks = doc.getMap("blocks");
				var ids = %s;
				blocks.forEach(function(b, bid) {
					if (!(b instanceof Y.Map)) return;
					if (b.get("sys:flavour") === "affine:note") {
						var children = b.get("sys:children");
						if (children instanceof Y.Array) {
							for (var i = 0; i < ids.length; i++) {
								children.push([ids[i]]);
							}
						}
					}
				});
				return "ok";
			})()
		`, engDocID, idsJS)
		_, err := s.Engine.RunScript(script)
		return err
	})
	return count, err
}

// ReplaceWithMarkdown replaces all content blocks with new markdown content.
func (s *Session) ReplaceWithMarkdown(docID, markdown string) (int, error) {
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return 0, err
	}

	blocks, err := s.Engine.ReadBlocks(engDocID)
	if err != nil {
		return 0, fmt.Errorf("read blocks for title strip: %w", err)
	}
	markdown = stripMirrorOfExportedDocTitle(markdown, pageTitleFromBlocks(blocks))

	lines := parseMarkdownToLines(markdown)

	count := 0
	err = s.PushDocDelta(engDocID, docID, func() error {
		// Remove existing content blocks (keep page, surface, note)
		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var blocks = doc.getMap("blocks");
				var toDelete = [];
				blocks.forEach(function(b, id) {
					if (!(b instanceof Y.Map)) return;
					var f = b.get("sys:flavour");
					if (f !== "affine:page" && f !== "affine:surface" && f !== "affine:note") {
						toDelete.push(id);
					}
				});
				for (var i = 0; i < toDelete.length; i++) {
					blocks.delete(toDelete[i]);
				}
				// Clear note children
				blocks.forEach(function(b, id) {
					if (!(b instanceof Y.Map)) return;
					if (b.get("sys:flavour") === "affine:note") {
						var children = b.get("sys:children");
						if (children instanceof Y.Array) {
							children.delete(0, children.length);
						}
					}
				});
				return toDelete.length;
			})()
		`, engDocID)
		_, err := s.Engine.RunScript(script)
		if err != nil {
			return err
		}

		// Create new blocks
		var blockIDs []string
		for _, line := range lines {
			blockID := "b-" + GenerateDocID()
			err := s.Engine.CreateFormattedBlock(engDocID, blockID, line.flavour, line.btype, line.text)
			if err != nil {
				return err
			}
			extra := ""
			if line.language != "" {
				extra = fmt.Sprintf(`block.set("prop:language", %q);`, line.language)
			}
			if line.checked {
				extra += `block.set("prop:checked", true);`
			}
			script := fmt.Sprintf(`
				(function() {
					var doc = globalThis._docs[%d];
					var block = doc.getMap("blocks").get(%q);
					block.set("sys:children", new Y.Array());
					%s
					return "ok";
				})()
			`, engDocID, blockID, extra)
			_, err = s.Engine.RunScript(script)
			if err != nil {
				return err
			}
			blockIDs = append(blockIDs, blockID)
			count++
		}

		// Add block IDs to note
		idsJS := `[`
		for i, id := range blockIDs {
			if i > 0 {
				idsJS += ","
			}
			idsJS += fmt.Sprintf("%q", id)
		}
		idsJS += `]`

		script = fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var blocks = doc.getMap("blocks");
				var ids = %s;
				blocks.forEach(function(b, bid) {
					if (!(b instanceof Y.Map)) return;
					if (b.get("sys:flavour") === "affine:note") {
						var children = b.get("sys:children");
						if (children instanceof Y.Array) {
							for (var i = 0; i < ids.length; i++) {
								children.push([ids[i]]);
							}
						}
					}
				});
				return "ok";
			})()
		`, engDocID, idsJS)
		_, err = s.Engine.RunScript(script)
		return err
	})
	return count, err
}

// CreateDocFromMarkdown creates a new doc and fills it with markdown content.
func (s *Session) CreateDocFromMarkdown(title, markdown string) (string, error) {
	docID, err := s.CreateDoc(title)
	if err != nil {
		return "", err
	}

	_, err = s.AppendMarkdown(docID, markdown)
	if err != nil {
		return docID, fmt.Errorf("doc created (%s) but content failed: %w", docID, err)
	}

	return docID, nil
}

// markdownLine represents a parsed markdown line.
type markdownLine struct {
	flavour  string
	btype    string
	text     string
	language string
	checked  bool
	// For table blocks
	tableHeaders []string
	tableRows    [][]string
}

// parseMarkdownToLines parses markdown into block descriptors.
func parseMarkdownToLines(md string) []markdownLine {
	var result []markdownLine
	lines := strings.Split(md, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			i++
			continue
		}

		// Code block
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimPrefix(trimmed, "```")
			lang = strings.TrimSpace(lang)
			var codeLines []string
			i++
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == "```" {
					i++
					break
				}
				codeLines = append(codeLines, lines[i])
				i++
			}
			result = append(result, markdownLine{
				flavour:  "affine:code",
				btype:    "",
				text:     strings.Join(codeLines, "\n"),
				language: lang,
			})
			continue
		}

		// Markdown table: detect header row with pipes
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			headers := parseTableRow(trimmed)
			// Check if next line is separator (|---|---|)
			if i+1 < len(lines) {
				sepLine := strings.TrimSpace(lines[i+1])
				if isTableSeparator(sepLine) {
					i += 2 // skip header + separator
					var rows [][]string
					for i < len(lines) {
						rowTrimmed := strings.TrimSpace(lines[i])
						if !strings.HasPrefix(rowTrimmed, "|") {
							break
						}
						rows = append(rows, parseTableRow(rowTrimmed))
						i++
					}
					result = append(result, markdownLine{
						flavour:      "affine:database",
						tableHeaders: headers,
						tableRows:    rows,
					})
					continue
				}
			}
		}

		// Divider
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			result = append(result, markdownLine{flavour: "affine:divider"})
			i++
			continue
		}

		// ATX headings: 1–6 leading '#' marks; space after hashes is optional (e.g. "# Title" or "#Title"). Adds h5/h6.
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
				level++
			}
			if level > 0 {
				title := strings.TrimSpace(trimmed[level:])
				levelTypes := []string{"h1", "h2", "h3", "h4", "h5", "h6"}
				result = append(result, markdownLine{
					flavour: "affine:paragraph",
					btype:   levelTypes[level-1],
					text:    title,
				})
				i++
				continue
			}
		}

		// Blockquote
		if strings.HasPrefix(trimmed, "> ") {
			result = append(result, markdownLine{flavour: "affine:paragraph", btype: "quote", text: strings.TrimPrefix(trimmed, "> ")})
			i++
			continue
		}

		// Todo list
		if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
			result = append(result, markdownLine{flavour: "affine:list", btype: "todo", text: trimmed[6:], checked: true})
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "- [ ] ") {
			result = append(result, markdownLine{flavour: "affine:list", btype: "todo", text: trimmed[6:]})
			i++
			continue
		}

		// Bulleted list
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			result = append(result, markdownLine{flavour: "affine:list", btype: "bulleted", text: trimmed[2:]})
			i++
			continue
		}

		// Numbered list
		if len(trimmed) > 2 {
			dotIdx := strings.Index(trimmed, ". ")
			if dotIdx > 0 && dotIdx <= 3 {
				allDigits := true
				for _, c := range trimmed[:dotIdx] {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					result = append(result, markdownLine{flavour: "affine:list", btype: "numbered", text: trimmed[dotIdx+2:]})
					i++
					continue
				}
			}
		}

		// Regular paragraph
		result = append(result, markdownLine{flavour: "affine:paragraph", btype: "text", text: trimmed})
		i++
	}
	return result
}

// createTableBlock creates an affine:table block from parsed markdown table data.
// Cell text is stored as Y.Text with inline markdown formatting support.
func createTableBlock(engine *yjs.Engine, engDocID int, blockID string, headers []string, rows [][]string) error {
	// Include header as first row
	allRows := append([][]string{headers}, rows...)
	numRows := len(allRows)
	numCols := len(headers)

	// Generate row/column IDs
	var rowIDs, colIDs []string
	for i := 0; i < numRows; i++ {
		rowIDs = append(rowIDs, "r-"+GenerateDocID())
	}
	for i := 0; i < numCols; i++ {
		colIDs = append(colIDs, "c-"+GenerateDocID())
	}

	// Build flat-key set statements for rows, columns, and cells
	var flatKeys []string
	for i, rid := range rowIDs {
		flatKeys = append(flatKeys,
			fmt.Sprintf(`block.set("prop:rows.%s.rowId", %q);`, rid, rid),
			fmt.Sprintf(`block.set("prop:rows.%s.order", "r%04d");`, rid, i),
		)
	}
	for i, cid := range colIDs {
		flatKeys = append(flatKeys,
			fmt.Sprintf(`block.set("prop:columns.%s.columnId", %q);`, cid, cid),
			fmt.Sprintf(`block.set("prop:columns.%s.order", "c%04d");`, cid, i),
		)
	}

	// Build cell data as JS array for inline markdown parsing
	var cellEntries []string
	for ri, row := range allRows {
		for ci := 0; ci < numCols; ci++ {
			text := ""
			if ci < len(row) {
				text = row[ci]
			}
			cellKey := fmt.Sprintf("prop:cells.%s:%s.text", rowIDs[ri], colIDs[ci])
			cellEntries = append(cellEntries, fmt.Sprintf(`{key:%q,text:%q}`, cellKey, text))
		}
	}

	script := fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var block = new Y.Map();
			block.set("sys:id", %q);
			block.set("sys:flavour", "affine:table");
			block.set("sys:version", 1);
			block.set("sys:parent", null);
			block.set("sys:children", new Y.Array());

			// Attach block to doc first (goja Y.js needs doc context for shared types)
			blocks.set(%q, block);

			// Set rows/columns as flat keys
			%s

			// Set cells as flat keys with Y.Text values + inline markdown
			var cellData = [%s];
			for (var i = 0; i < cellData.length; i++) {
				var cd = cellData[i];
				var segments = parseInlineMarkdown(cd.text);
				var yText = new Y.Text();
				var pos = 0;
				for (var j = 0; j < segments.length; j++) {
					var seg = segments[j];
					yText.insert(pos, seg.text, seg.attrs || {});
					pos += seg.text.length;
				}
				block.set(cd.key, yText);
			}

			block.set("prop:comments", undefined);
			block.set("prop:textAlign", undefined);
			return "ok";
		})()
	`, engDocID, blockID, blockID,
		strings.Join(flatKeys, "\n\t\t\t"),
		strings.Join(cellEntries, ","))

	_, err := engine.RunScript(script)
	return err
}

// AddTableRow adds a row to an existing affine:table block using flat-key format.
// It reads existing column IDs and row count, then appends a new row.
func (s *Session) AddTableRow(docID, tableBlockID string, cells []string) (string, error) {
	engDocID, err := s.LoadDoc(docID)
	if err != nil {
		return "", err
	}

	rowID := "r-" + GenerateDocID()

	// Build cell texts as JSON array
	var cellJSON []string
	for i, text := range cells {
		cellJSON = append(cellJSON, fmt.Sprintf(`{idx:%d,text:%q}`, i, text))
	}

	err = s.PushDocDelta(engDocID, docID, func() error {
		script := fmt.Sprintf(`
			(function() {
				var doc = globalThis._docs[%d];
				var blocks = doc.getMap("blocks");
				var block = blocks.get(%q);
				if (!block) return "error: table block not found";

				// Collect existing column IDs in order
				var colEntries = [];
				var maxRowOrder = -1;
				for (var key of block.keys()) {
					var cm = key.match(/^prop:columns\.([^.]+)\.order$/);
					if (cm) {
						colEntries.push({id: cm[1], order: block.get(key)});
					}
					var rm = key.match(/^prop:rows\.([^.]+)\.order$/);
					if (rm) {
						var ord = block.get(key);
						var num = parseInt(ord.replace("r",""), 10);
						if (num > maxRowOrder) maxRowOrder = num;
					}
				}
				colEntries.sort(function(a,b) { return a.order.localeCompare(b.order); });
				var newOrder = "r" + String(maxRowOrder + 1).padStart(4, "0");

				// Add new row
				block.set("prop:rows.%s.rowId", %q);
				block.set("prop:rows.%s.order", newOrder);

				// Add cells
				var cellData = [%s];
				for (var i = 0; i < cellData.length; i++) {
					if (i >= colEntries.length) break;
					var colId = colEntries[i].id;
					var segments = parseInlineMarkdown(cellData[i].text);
					var yText = new Y.Text();
					var pos = 0;
					for (var j = 0; j < segments.length; j++) {
						var seg = segments[j];
						yText.insert(pos, seg.text, seg.attrs || {});
						pos += seg.text.length;
					}
					block.set("prop:cells." + %q + ":" + colId + ".text", yText);
				}
				return "ok";
			})()
		`, engDocID, tableBlockID, rowID, rowID, rowID, strings.Join(cellJSON, ","), rowID)
		_, err := s.Engine.RunScript(script)
		return err
	})

	return rowID, err
}

// parseTableRow extracts cell values from a markdown table row like "| a | b | c |"
func parseTableRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.Trim(row, "|")
	parts := strings.Split(row, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// isTableSeparator checks if a line is a markdown table separator like "|---|---|"
func isTableSeparator(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return false
	}
	cleaned := strings.ReplaceAll(line, "|", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned == ""
}
