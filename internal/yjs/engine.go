package yjs

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

//go:embed yjs_bundle.js
var yjsBundle string

// Engine wraps a goja JS runtime with Y.js loaded.
type Engine struct {
	vm *goja.Runtime
	mu sync.Mutex
}

// NewEngine creates a new Y.js engine with the bundled library loaded.
func NewEngine() (*Engine, error) {
	vm := goja.New()

	// Provide browser API shims that goja doesn't have
	vm.RunString(`
		var console = {
			log: function() {},
			warn: function() {},
			error: function() {},
			info: function() {},
			debug: function() {}
		};
		var crypto = {
			getRandomValues: function(arr) {
				for (var i = 0; i < arr.length; i++) {
					arr[i] = Math.floor(Math.random() * 256);
				}
				return arr;
			}
		};

		var _b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
		function btoa(str) {
			var out = "";
			for (var i = 0; i < str.length; i += 3) {
				var c1 = str.charCodeAt(i);
				var c2 = i + 1 < str.length ? str.charCodeAt(i + 1) : 0;
				var c3 = i + 2 < str.length ? str.charCodeAt(i + 2) : 0;
				out += _b64chars[(c1 >> 2) & 63];
				out += _b64chars[((c1 << 4) | (c2 >> 4)) & 63];
				out += i + 1 < str.length ? _b64chars[((c2 << 2) | (c3 >> 6)) & 63] : "=";
				out += i + 2 < str.length ? _b64chars[c3 & 63] : "=";
			}
			return out;
		}
		function atob(str) {
			str = str.replace(/=+$/, "");
			var out = "";
			for (var i = 0; i < str.length; i += 4) {
				var b1 = _b64chars.indexOf(str[i]);
				var b2 = i + 1 < str.length ? _b64chars.indexOf(str[i + 1]) : 0;
				var b3 = i + 2 < str.length ? _b64chars.indexOf(str[i + 2]) : 0;
				var b4 = i + 3 < str.length ? _b64chars.indexOf(str[i + 3]) : 0;
				out += String.fromCharCode(((b1 << 2) | (b2 >> 4)) & 255);
				if (i + 2 < str.length + (str.length % 4 ? 4 - str.length % 4 : 0)) out += String.fromCharCode(((b2 << 4) | (b3 >> 2)) & 255);
				if (i + 3 < str.length + (str.length % 4 ? 4 - str.length % 4 : 0)) out += String.fromCharCode(((b3 << 6) | b4) & 255);
			}
			return out;
		}
	`)

	// Load Y.js bundle
	_, err := vm.RunString(yjsBundle)
	if err != nil {
		return nil, fmt.Errorf("load yjs bundle: %w", err)
	}

	// Initialize docs array and inline markdown parser
	vm.RunString(`globalThis._docs = [];`)

	// Register inline markdown parser: **bold**, *italic*, ` + "`code`" + `, ~~strike~~, [text](url),
	// ==bg:…::…==, ==fg:…::…==, ==bg:…|fg:…::…== (AFFiNE background / text color), ==shortcut== (bg: yellow).
	// Export uses deltaToAffineMarkdown with the same ==-prefix forms.
	_, err = vm.RunString(`
		function findClosingHighlightEquals(src, start) {
			var p = start;
			while (p < src.length) {
				var q = src.indexOf('==', p);
				if (q === -1) return -1;
				var bs = 0;
				for (var k = q - 1; k >= start && src[k] === '\\'; k--) bs++;
				if (bs % 2 === 0) return q;
				p = q + 2;
			}
			return -1;
		}

		function emitHighlightPrefix(attrs) {
			var parts = [];
			if (attrs.background) parts.push('bg:' + String(attrs.background));
			if (attrs.color) parts.push('fg:' + String(attrs.color));
			return parts.length ? parts.join('|') : '';
		}

		function wrapHighlightMarkdown(inner, attrs) {
			if (!attrs || (!attrs.background && !attrs.color)) return inner;
			var pre = emitHighlightPrefix(attrs);
			inner = inner.split('==').join('\\=\\=');
			return '==' + pre + '::' + inner + '==';
		}

		function emitDeltaRun(text, attrs) {
			if (!attrs) attrs = {};
			if (attrs.reference && typeof attrs.reference === 'object') return text;
			if (attrs.code) {
				var cbody = text.replace(/` + "`" + `/g, '\\` + "`" + `');
				var part = '` + "`" + `' + cbody + '` + "`" + `';
				var hi = {};
				if (attrs.background) hi.background = attrs.background;
				if (attrs.color) hi.color = attrs.color;
				return wrapHighlightMarkdown(part, hi);
			}
			var inner = text;
			if (attrs.strike) inner = '~~' + inner + '~~';
			if (attrs.italic) inner = '*' + inner + '*';
			if (attrs.bold) inner = '**' + inner + '**';
			if (attrs.link) {
				var u = String(attrs.link).replace(/\)/g, '\\)');
				inner = '[' + inner + '](' + u + ')';
			}
			var hi2 = {};
			if (attrs.background) hi2.background = attrs.background;
			if (attrs.color) hi2.color = attrs.color;
			return wrapHighlightMarkdown(inner, hi2);
		}

		function deltaToAffineMarkdown(yText) {
			if (!(yText instanceof Y.Text)) return '';
			var raw = yText.toDelta();
			var merged = [];
			for (var di = 0; di < raw.length; di++) {
				var op = raw[di];
				if (typeof op.insert !== 'string' || op.insert === '') continue;
				var attrs = op.attributes || {};
				var acopy = {};
				for (var ak in attrs) acopy[ak] = attrs[ak];
				if (merged.length > 0) {
					var prev = merged[merged.length - 1];
					if (JSON.stringify(prev.attributes) === JSON.stringify(acopy)) {
						prev.insert += op.insert;
						continue;
					}
				}
				merged.push({insert: op.insert, attributes: acopy});
			}
			var out = '';
			for (var mi = 0; mi < merged.length; mi++) {
				out += emitDeltaRun(merged[mi].insert, merged[mi].attributes);
			}
			return out;
		}

		function parseInlineMarkdown(src) {
			var segments = [];
			var i = 0;
			var buf = "";

			function flush() {
				if (buf.length > 0) { segments.push({text: buf, attrs: {}}); buf = ""; }
			}

			while (i < src.length) {
				// Escaped character
				if (src[i] === '\\' && i + 1 < src.length) {
					buf += src[i + 1];
					i += 2;
					continue;
				}

				// Inline code: ` + "`...`" + `
				if (src[i] === '` + "`" + `') {
					var end = src.indexOf('` + "`" + `', i + 1);
					if (end !== -1) {
						flush();
						segments.push({text: src.substring(i + 1, end), attrs: {code: true}});
						i = end + 1;
						continue;
					}
				}

				// Highlight / text color: ==bg:C::b==, ==fg:C::b==, ==bg:C|fg:D::b==, ==b== (shorthand → bg yellow)
				if (src[i] === '=' && i + 1 < src.length && src[i + 1] === '=') {
					var rest = i + 2;
					var bodyStart = rest;
					var explicitPrefix = null;
					var sep = src.indexOf('::', rest);
					if (sep !== -1) {
						var maybePre = src.substring(rest, sep);
						if (/^(bg:|fg:)/.test(maybePre)) {
							explicitPrefix = maybePre;
							bodyStart = sep + 2;
						}
					}
					var endHi = findClosingHighlightEquals(src, bodyStart);
					if (endHi !== -1) {
						flush();
						var innerRaw = src.substring(bodyStart, endHi);
						var innerSegs = parseInlineMarkdown(innerRaw);
						var bgVal = null;
						var fgVal = null;
						if (explicitPrefix) {
							var pcs = explicitPrefix.split('|');
							for (var pi = 0; pi < pcs.length; pi++) {
								var p = pcs[pi];
								if (p.indexOf('bg:') === 0) bgVal = p.slice(3) || 'yellow';
								else if (p.indexOf('fg:') === 0) fgVal = p.slice(3) || '';
							}
						} else {
							bgVal = 'yellow';
						}
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							if (bgVal !== null) a.background = bgVal;
							if (fgVal !== null && fgVal !== '') a.color = fgVal;
							segments.push({text: s.text, attrs: a});
						}
						i = endHi + 2;
						continue;
					}
				}

				// Bold: **...**
				if (src[i] === '*' && i + 1 < src.length && src[i + 1] === '*') {
					var end = src.indexOf('**', i + 2);
					if (end !== -1) {
						flush();
						// Recursively parse inner content for nested formatting
						var inner = src.substring(i + 2, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.bold = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 2;
						continue;
					}
				}

				// Strikethrough: ~~...~~
				if (src[i] === '~' && i + 1 < src.length && src[i + 1] === '~') {
					var end = src.indexOf('~~', i + 2);
					if (end !== -1) {
						flush();
						var inner = src.substring(i + 2, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.strike = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 2;
						continue;
					}
				}

				// Italic: *...*  (single asterisk, not followed by another)
				if (src[i] === '*' && (i + 1 >= src.length || src[i + 1] !== '*')) {
					var end = -1;
					for (var si = i + 1; si < src.length; si++) {
						if (src[si] === '*' && (si + 1 >= src.length || src[si + 1] !== '*') && (si === 0 || src[si - 1] !== '*')) {
							end = si; break;
						}
					}
					if (end !== -1) {
						flush();
						var inner = src.substring(i + 1, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.italic = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 1;
						continue;
					}
				}

				// Link: [text](url)
				if (src[i] === '[') {
					var closeBracket = src.indexOf(']', i + 1);
					if (closeBracket !== -1 && closeBracket + 1 < src.length && src[closeBracket + 1] === '(') {
						var closeParen = src.indexOf(')', closeBracket + 2);
						if (closeParen !== -1) {
							flush();
							var linkText = src.substring(i + 1, closeBracket);
							var linkUrl = src.substring(closeBracket + 2, closeParen);
							segments.push({text: linkText, attrs: {link: linkUrl}});
							i = closeParen + 1;
							continue;
						}
					}
				}

				buf += src[i];
				i++;
			}
			flush();
			return segments;
		}
	`)
	if err != nil {
		return nil, fmt.Errorf("load markdown parser: %w", err)
	}

	return &Engine{vm: vm}, nil
}

// ApplyBase64Update creates a Y.Doc and applies a base64-encoded update.
// Returns the doc handle ID.
func (e *Engine) ApplyBase64Update(b64 string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0, fmt.Errorf("decode base64: %w", err)
	}

	e.vm.Set("_updateBytes", raw)
	val, err := e.vm.RunString(`
		(function() {
			var doc = new Y.Doc();
			var arr = new Uint8Array(_updateBytes);
			Y.applyUpdate(doc, arr);
			var id = globalThis._docs.length;
			globalThis._docs.push(doc);
			return id;
		})()
	`)
	if err != nil {
		return 0, fmt.Errorf("apply update: %w", err)
	}
	return int(val.ToInteger()), nil
}

// ApplyUpdate applies a raw Yjs update to a new Y.Doc and returns its handle ID.
func (e *Engine) ApplyUpdate(raw []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_updateBytes", raw)
	val, err := e.vm.RunString(`
		(function() {
			var doc = new Y.Doc();
			var arr = new Uint8Array(_updateBytes);
			Y.applyUpdate(doc, arr);
			var id = globalThis._docs.length;
			globalThis._docs.push(doc);
			return id;
		})()
	`)
	if err != nil {
		return 0, fmt.Errorf("apply update: %w", err)
	}
	return int(val.ToInteger()), nil
}

// WorkspacePageTitles reads page id → display title from a workspace root Y.Doc
// and optional docProperties Y.Doc (use propsDocID < 0 when the latter is unavailable).
// Matches AFFiNE client behaviour: docProperties title overrides meta.pages entry.
func (e *Engine) WorkspacePageTitles(workspaceDocID, propsDocID int) (map[string]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_wsDocId", workspaceDocID)
	e.vm.Set("_propsDocId", propsDocID)
	val, err := e.vm.RunString(`
		(function() {
			var ws = globalThis._docs[_wsDocId];
			if (!ws) return "{}";
			var metaMap = ws.getMap("meta");
			var meta = metaMap.toJSON ? metaMap.toJSON() : {};
			var pages = Array.isArray(meta.pages) ? meta.pages : [];
			var propsDoc = (_propsDocId >= 0) ? globalThis._docs[_propsDocId] : null;
			var out = {};
			function propsMapForPageId(propsRoot, pageId) {
				if (!propsRoot || !pageId || !propsRoot.share || typeof propsRoot.share.get !== 'function') return null;
				var sid = typeof pageId === 'string' ? pageId : String(pageId);
				var t = propsRoot.share.get(sid);
				if (t && t instanceof Y.Map) return t;
				return null;
			}
			for (var i = 0; i < pages.length; i++) {
				var p = pages[i];
				if (!p || p.trash) continue;
				var id = p.id;
				if (!id) continue;
				var title = null;
				if (propsDoc) {
					var pm = propsMapForPageId(propsDoc, id);
					if (pm && pm.toJSON) {
						var props = pm.toJSON();
						if (props && props.title != null && props.title !== "") title = props.title;
					}
				}
				if (title == null || title === "") title = p.title;
				if (typeof title !== "string") {
					if (title && typeof title === "object" && title.value) title = title.value;
					else title = "";
				}
				out[id] = title || "";
			}
			return JSON.stringify(out);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("workspace page titles: %w", err)
	}
	result := make(map[string]string)
	if err := json.Unmarshal([]byte(val.String()), &result); err != nil {
		return nil, fmt.Errorf("parse title map: %w", err)
	}
	return result, nil
}

// WorkspacePageListJSON returns a JSON array of pages: id, title, parentId, createDate.
// Trashed pages are omitted. Titles merge docProperties like WorkspacePageTitles.
func (e *Engine) WorkspacePageListJSON(workspaceDocID, propsDocID int) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_wsDocId", workspaceDocID)
	e.vm.Set("_propsDocId", propsDocID)
	val, err := e.vm.RunString(`
		(function() {
			var ws = globalThis._docs[_wsDocId];
			if (!ws) return "[]";
			var metaMap = ws.getMap("meta");
			var propsDoc = (_propsDocId >= 0) ? globalThis._docs[_propsDocId] : null;
			var out = [];

			function normalizeTitle(title) {
				if (typeof title !== "string") {
					if (title && typeof title === "object" && title.value) title = title.value;
					else title = "";
				}
				return title || "";
			}

			function propsMapForPageId(propsRoot, pageId) {
				if (!propsRoot || !pageId || !propsRoot.share || typeof propsRoot.share.get !== "function") return null;
				var sid = typeof pageId === "string" ? pageId : String(pageId);
				var t = propsRoot.share.get(sid);
				if (t && t instanceof Y.Map) return t;
				return null;
			}

			function pushPage(p) {
				if (!p || !p.id || p.trash) return;
				var id = typeof p.id === "string" ? p.id : String(p.id);
				var title = null;
				if (propsDoc) {
					var propMap = propsMapForPageId(propsDoc, id);
					if (propMap && propMap.toJSON) {
						var props = propMap.toJSON();
						if (props && props.title != null && props.title !== "") title = props.title;
					}
				}
				if (title == null || title === "") title = p.title;
				title = normalizeTitle(title);
				var parentId = p.parentId || p.parent || null;
				if (parentId != null && parentId !== "") parentId = String(parentId);
				else parentId = null;
				out.push({
					id: id,
					title: title,
					parentId: parentId,
					createDate: p.createDate || 0
				});
			}

			function yMapToPageRecord(m) {
				if (!m || !(m instanceof Y.Map)) return null;
				var o = {};
				m.forEach(function(val, key) {
					if (val instanceof Y.Text) o[key] = val.toString();
					else if (typeof val === "string" || typeof val === "number" || typeof val === "boolean") o[key] = val;
				});
				return o;
			}

			// Prefer live Y.Array/Y.Map iteration: meta.toJSON() can omit parentId that exists on Y.Map (AFFiNE sidebar tree).
			var pagesArr = metaMap.get("pages");
			if (pagesArr != null && typeof pagesArr.forEach === "function") {
				pagesArr.forEach(function(item) {
					var p = yMapToPageRecord(item);
					if (p) pushPage(p);
				});
			} else {
				var meta = metaMap.toJSON ? metaMap.toJSON() : {};
				var pages = Array.isArray(meta.pages) ? meta.pages : [];
				for (var i = 0; i < pages.length; i++) {
					pushPage(pages[i]);
				}
			}
			return JSON.stringify(out);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("workspace page list: %w", err)
	}
	return []byte(val.String()), nil
}

// ReadBlocks reads all blocks from a Y.Doc and returns them as JSON-friendly map.
func (e *Engine) ReadBlocks(docID int) (map[string]map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var result = {};
			blocks.forEach(function(block, blockId) {
				if (!(block instanceof Y.Map)) return;
				var b = {};
				block.forEach(function(v, k) {
					if (v instanceof Y.Text) b[k] = v.toString();
					else if (v instanceof Y.Array) {
						var arr = [];
						v.forEach(function(item) {
							if (typeof item === "string") arr.push(item);
							else if (item instanceof Y.Map) {
								var obj = {};
								item.forEach(function(val, key) {
									if (val instanceof Y.Text) obj[key] = val.toString();
									else obj[key] = val;
								});
								arr.push(obj);
							} else {
								arr.push(item);
							}
						});
						b[k] = arr;
					} else if (v instanceof Y.Map) {
						var obj = {};
						v.forEach(function(val, key) {
							if (val instanceof Y.Text) obj[key] = val.toString();
							else obj[key] = val;
						});
						b[k] = obj;
					} else {
						b[k] = v;
					}
				});
				result[blockId] = b;
			});
			return JSON.stringify(result);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("readBlocks: %w", err)
	}

	result := make(map[string]map[string]any)
	if err := json.Unmarshal([]byte(val.String()), &result); err != nil {
		return nil, fmt.Errorf("parse blocks JSON: %w", err)
	}
	return result, nil
}

// ReadMeta reads the "meta" Y.Map from a document.
func (e *Engine) ReadMeta(docID int) (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var meta = doc.getMap("meta");
			var result = {};
			meta.forEach(function(v, k) {
				if (v instanceof Y.Text) result[k] = v.toString();
				else if (v instanceof Y.Array) {
					var arr = [];
					v.forEach(function(item) { arr.push(item); });
					result[k] = arr;
				} else {
					result[k] = v;
				}
			});
			return JSON.stringify(result);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("readMeta: %w", err)
	}
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(val.String()), &result); err != nil {
		return nil, fmt.Errorf("parse meta JSON: %w", err)
	}
	return result, nil
}

// NewDoc creates a new empty Y.Doc and returns its handle ID.
func (e *Engine) NewDoc() (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	val, err := e.vm.RunString(`
		(function() {
			var doc = new Y.Doc();
			var id = globalThis._docs.length;
			globalThis._docs.push(doc);
			return id;
		})()
	`)
	if err != nil {
		return 0, fmt.Errorf("newDoc: %w", err)
	}
	return int(val.ToInteger()), nil
}

// EncodeStateAsUpdate encodes the full state as base64.
func (e *Engine) EncodeStateAsUpdate(docID int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var update = Y.encodeStateAsUpdate(doc);
			var binary = '';
			for (var i = 0; i < update.length; i++) {
				binary += String.fromCharCode(update[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("encodeStateAsUpdate: %w", err)
	}
	return val.String(), nil
}

// SaveStateVector returns the state vector as base64.
func (e *Engine) SaveStateVector(docID int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var sv = Y.encodeStateVector(doc);
			var binary = '';
			for (var i = 0; i < sv.length; i++) {
				binary += String.fromCharCode(sv[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("saveStateVector: %w", err)
	}
	return val.String(), nil
}

// EncodeDelta encodes the delta from a saved state vector as base64.
func (e *Engine) EncodeDelta(docID int, stateVectorB64 string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_svB64", stateVectorB64)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var svBin = atob(_svB64);
			var sv = new Uint8Array(svBin.length);
			for (var i = 0; i < svBin.length; i++) sv[i] = svBin.charCodeAt(i);
			var update = Y.encodeStateAsUpdate(doc, sv);
			var binary = '';
			for (var i = 0; i < update.length; i++) {
				binary += String.fromCharCode(update[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("encodeDelta: %w", err)
	}
	return val.String(), nil
}

// InsertFormattedText creates a Y.Text with inline markdown formatting
// converted to proper Y.Text attributes (bold, italic, code, etc.).
// Returns the text content for verification.
func (e *Engine) InsertFormattedText(docID int, blockID, key, markdown string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	e.vm.Set("_key", key)
	e.vm.Set("_markdown", markdown)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var block = blocks.get(_blockId);
			if (!block) return "error: block not found";

			var text = new Y.Text();
			var segments = parseInlineMarkdown(_markdown);
			var pos = 0;
			for (var i = 0; i < segments.length; i++) {
				var seg = segments[i];
				text.insert(pos, seg.text, seg.attrs || {});
				pos += seg.text.length;
			}
			block.set(_key, text);
			return text.toString();
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("insertFormattedText: %w", err)
	}
	return val.String(), nil
}

// CreateFormattedBlock creates a new block with properly formatted rich text.
func (e *Engine) CreateFormattedBlock(docID int, blockID, flavour, blockType, markdownText string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	e.vm.Set("_flavour", flavour)
	e.vm.Set("_blockType", blockType)
	e.vm.Set("_markdown", markdownText)
	_, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var block = new Y.Map();
			block.set("sys:id", _blockId);
			block.set("sys:flavour", _flavour);
			block.set("sys:version", 1);
			block.set("sys:children", new Y.Array());
			// Match native BlockSuite/AFFiNE: paragraph and list types live in prop:type only (not sys:type).
			if (_flavour === "affine:paragraph") {
				block.set("prop:type", _blockType || "text");
				block.set("prop:collapsed", false);
			} else if (_flavour === "affine:list" && _blockType) {
				block.set("prop:type", _blockType);
			}

			var text = new Y.Text();
			var segments = parseInlineMarkdown(_markdown);
			var pos = 0;
			for (var i = 0; i < segments.length; i++) {
				var seg = segments[i];
				text.insert(pos, seg.text, seg.attrs || {});
				pos += seg.text.length;
			}
			block.set("prop:text", text);
			blocks.set(_blockId, block);
			return "ok";
		})()
	`)
	if err != nil {
		return fmt.Errorf("createFormattedBlock: %w", err)
	}
	return nil
}

// NoteChildOrder returns top-level content block IDs in document order (affine:note → sys:children).
func (e *Engine) NoteChildOrder(docID int) ([]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var order = [];
			blocks.forEach(function(block, id) {
				if (!(block instanceof Y.Map)) return;
				if (block.get("sys:flavour") === "affine:note") {
					var children = block.get("sys:children");
					if (children instanceof Y.Array) {
						children.forEach(function(childId) {
							order.push(String(childId));
						});
					}
				}
			});
			return JSON.stringify(order);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("noteChildOrder: %w", err)
	}
	var order []string
	if err := json.Unmarshal([]byte(val.String()), &order); err != nil {
		return nil, fmt.Errorf("noteChildOrder parse: %w", err)
	}
	return order, nil
}

// BlockPropTypeString reads prop:type (then sys:type) from the live Y block with coercion.
func (e *Engine) BlockPropTypeString(docID int, blockID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	val, err := e.vm.RunString(`
		(function() {
			var NATIVE = "$blocksuite:internal:native$";
			function isYMapLike(v) {
				return v != null && typeof v.get === "function";
			}
			function coerce(v) {
				if (v === undefined || v === null) return "";
				if (typeof v === "string") return v;
				if (typeof v === "number") {
					var n = Math.floor(v);
					if (n >= 1 && n <= 6 && n === v) return "h" + n;
					return "";
				}
				if (v instanceof Y.Text) return v.toString();
				if (v instanceof Y.Map || isYMapLike(v)) {
					var tag = v.get("type");
					if (tag === NATIVE) return coerce(v.get("value"));
					var innerVal = v.get("value");
					if (innerVal !== undefined && innerVal !== null) return coerce(innerVal);
					if (typeof tag === "string" && tag !== NATIVE) return coerce(tag);
					return "";
				}
				if (typeof v === "object") {
					if (v.value !== undefined && v.value !== null) return coerce(v.value);
					if (typeof v.type === "string" && v.type !== NATIVE) return coerce(v.type);
				}
				return "";
			}
			function preferHeading(candidates) {
				for (var i = 0; i < candidates.length; i++) {
					if (/^h[1-6]$/.test(candidates[i])) return candidates[i];
				}
				return candidates.length ? candidates[0] : "";
			}
			var doc = globalThis._docs[_docId];
			var block = doc.getMap("blocks").get(_blockId);
			if (!block || !isYMapLike(block)) return "";
			var seen = {};
			var list = [];
			function push(u) {
				if (u === "" || seen[u]) return;
				seen[u] = true;
				list.push(u);
			}
			push(coerce(block.get("prop:type")));
			push(coerce(block.get("prop:type.value")));
			block.forEach(function(val, key) {
				if (typeof key !== "string") return;
				if (key.indexOf("prop:type") !== 0) return;
				push(coerce(val));
			});
			var t = preferHeading(list);
			if (t !== "") return t;
			t = coerce(block.get("sys:type"));
			if (t !== "") return t;
			t = coerce(block.get("prop:headingLevel"));
			if (t !== "") return t;
			return "";
		})()
	`)
	if err != nil {
		return ""
	}
	return val.String()
}

// BlockPropTextString returns plain text from prop:text when it is a Y.Text.
func (e *Engine) BlockPropTextString(docID int, blockID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var block = doc.getMap("blocks").get(_blockId);
			if (!block || !(block instanceof Y.Map)) return "";
			var t = block.get("prop:text");
			if (t instanceof Y.Text) return t.toString();
			return "";
		})()
	`)
	if err != nil {
		return ""
	}
	return val.String()
}

// BlockPropTextAffineMarkdown serializes prop:text (Y.Text) to markdown that preserves
// inline attributes AFFiNE understands — notably ==bg:color::…== for background/highlight.
func (e *Engine) BlockPropTextAffineMarkdown(docID int, blockID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var block = doc.getMap("blocks").get(_blockId);
			if (!block || !(block instanceof Y.Map)) return "";
			var t = block.get("prop:text");
			if (!(t instanceof Y.Text)) return "";
			return deltaToAffineMarkdown(t);
		})()
	`)
	if err != nil {
		return ""
	}
	return val.String()
}

// RunScript executes arbitrary JS with access to Y and the docs array.
func (e *Engine) RunScript(script string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	val, err := e.vm.RunString(script)
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// FreeDoc removes a doc reference to allow GC.
func (e *Engine) FreeDoc(docID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.vm.Set("_docId", docID)
	e.vm.RunString(`globalThis._docs[_docId] = null;`)
}
