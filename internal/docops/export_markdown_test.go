package docops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tomohiro-owada/affine-cli/internal/yjs"
)

const exportHeadingGolden = "# proper header"

// minimalDocWithParagraphJS returns a script that builds page → note → one paragraph "proper header"
// and a placeholder COMMENT where the caller inserts prop:type setup lines.
func minimalDocWithParagraphJS(propTypeSetup string) string {
	return fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");

			var page = new Y.Map();
			page.set("sys:id", "page-root");
			page.set("sys:flavour", "affine:page");
			page.set("sys:version", 2);
			var pageTitle = new Y.Text();
			page.set("prop:title", pageTitle);
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
			var noteCh = new Y.Array();
			noteCh.push(["ph1"]);
			note.set("sys:children", noteCh);
			blocks.set("note-1", note);

			var para = new Y.Map();
			para.set("sys:id", "ph1");
			para.set("sys:flavour", "affine:paragraph");
			para.set("sys:version", 1);
			para.set("sys:children", new Y.Array());
			var ptext = new Y.Text();
			ptext.insert(0, "proper header");
			para.set("prop:text", ptext);
			%s
			blocks.set("ph1", para);
			return "ok";
		})()
	`, propTypeSetup)
}

func TestExportMarkdown_headingNativeBoxedPropType(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	setup := `
			var boxed = new Y.Map();
			boxed.set("type", "$blocksuite:internal:native$");
			boxed.set("value", "h1");
			para.set("prop:type", boxed);
	`
	if _, err := e.RunScript(minimalDocWithParagraphJS(setup)); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	if !strings.Contains(md, exportHeadingGolden) {
		t.Fatalf("markdown missing %q:\n%s", exportHeadingGolden, md)
	}
}

func TestExportMarkdown_headingFlatPropTypeKeys(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	setup := `
			para.set("prop:type.type", "$blocksuite:internal:native$");
			para.set("prop:type.value", "h1");
	`
	if _, err := e.RunScript(minimalDocWithParagraphJS(setup)); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	if !strings.Contains(md, exportHeadingGolden) {
		t.Fatalf("markdown missing %q:\n%s", exportHeadingGolden, md)
	}
}

func TestExportMarkdown_headingPlainStringPropType(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	setup := `para.set("prop:type", "h1");`
	if _, err := e.RunScript(minimalDocWithParagraphJS(setup)); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	if !strings.Contains(md, exportHeadingGolden) {
		t.Fatalf("markdown missing %q:\n%s", exportHeadingGolden, md)
	}
}

func TestExportMarkdown_headingNestedUnderParagraph(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	script := `
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");

			var page = new Y.Map();
			page.set("sys:id", "page-root");
			page.set("sys:flavour", "affine:page");
			page.set("sys:version", 2);
			page.set("prop:title", new Y.Text());
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
			var noteCh = new Y.Array();
			noteCh.push(["outer-p"]);
			note.set("sys:children", noteCh);
			blocks.set("note-1", note);

			var outer = new Y.Map();
			outer.set("sys:id", "outer-p");
			outer.set("sys:flavour", "affine:paragraph");
			outer.set("sys:version", 1);
			var outerKids = new Y.Array();
			outerKids.push(["inner-h"]);
			outer.set("sys:children", outerKids);
			var outerText = new Y.Text();
			outerText.insert(0, "wrapper");
			outer.set("prop:text", outerText);
			var outerBox = new Y.Map();
			outerBox.set("type", "$blocksuite:internal:native$");
			outerBox.set("value", "text");
			outer.set("prop:type", outerBox);
			blocks.set("outer-p", outer);

			var inner = new Y.Map();
			inner.set("sys:id", "inner-h");
			inner.set("sys:flavour", "affine:paragraph");
			inner.set("sys:version", 1);
			inner.set("sys:children", new Y.Array());
			var innerText = new Y.Text();
			innerText.insert(0, "proper header");
			inner.set("prop:text", innerText);
			var innerBox = new Y.Map();
			innerBox.set("type", "$blocksuite:internal:native$");
			innerBox.set("value", "h1");
			inner.set("prop:type", innerBox);
			blocks.set("inner-h", inner);

			return "ok";
		})()
	`
	if _, err := e.RunScript(script); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	if !strings.Contains(md, exportHeadingGolden) {
		t.Fatalf("markdown missing %q:\n%s", exportHeadingGolden, md)
	}
}

func TestExportMarkdown_todoLineWithGreenHighlight(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	script := `
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");

			var page = new Y.Map();
			page.set("sys:id", "page-root");
			page.set("sys:flavour", "affine:page");
			page.set("sys:version", 2);
			var pageTitle = new Y.Text();
			page.set("prop:title", pageTitle);
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
			var noteCh = new Y.Array();
			noteCh.push(["list1"]);
			note.set("sys:children", noteCh);
			blocks.set("note-1", note);

			var list = new Y.Map();
			list.set("sys:id", "list1");
			list.set("sys:flavour", "affine:list");
			list.set("sys:version", 1);
			list.set("sys:children", new Y.Array());
			list.set("prop:type", "todo");
			var lt = new Y.Text();
			lt.insert(0, "Populate JZ spreadsheet ");
			lt.insert(24, "[2]", { background: "green" });
			list.set("prop:text", lt);
			blocks.set("list1", list);
			return "ok";
		})()
	`
	if _, err := e.RunScript(script); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	want := "==bg:green::[2]=="
	if !strings.Contains(md, want) {
		t.Fatalf("markdown missing highlight %q:\n%s", want, md)
	}
	if !strings.Contains(md, "Populate JZ spreadsheet") {
		t.Fatalf("markdown missing plain prefix:\n%s", md)
	}
}

func TestExportMarkdown_headingPropTypeAsYText(t *testing.T) {
	e, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := e.NewDoc()
	setup := `
			var tH = new Y.Text();
			tH.insert(0, "h1");
			para.set("prop:type", tH);
	`
	if _, err := e.RunScript(minimalDocWithParagraphJS(setup)); err != nil {
		t.Fatal(err)
	}
	blocks, err := e.ReadBlocks(docID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := e.NoteChildOrder(docID)
	if err != nil {
		t.Fatal(err)
	}
	md := buildMarkdownExport(e, docID, blocks, order)
	if !strings.Contains(md, exportHeadingGolden) {
		t.Fatalf("markdown missing %q:\n%s", exportHeadingGolden, md)
	}
}
