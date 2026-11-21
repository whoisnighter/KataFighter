// Monaco setup for the code editor: the CNS and ZSS languages, a theme
// built from the CSS tokens, and the open documents (one model per file, so
// undo history and cursor survive tab switches).
//
// Only the editor core is bundled (no TypeScript/JSON/CSS language
// services). Checking comes from the Go side (EditorService.Check, which
// runs katafighter-lsp), never from code here.
import * as monaco from "monaco-editor/editor/editor.api";
import EditorWorker from "monaco-editor/editor/editor.worker?worker";
import { EditorService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";

export { monaco };

(self as any).MonacoEnvironment = { getWorker: () => new EditorWorker() };

// --- Languages ---------------------------------------------------------------
// Syntax colors only. They are generic on purpose: controller and trigger
// names come from lsp/schema through the Go side, not from lists here.

monaco.languages.register({ id: "cns", extensions: [".cns", ".st", ".cmd", ".const"], aliases: ["CNS"] });
monaco.languages.setLanguageConfiguration("cns", {
  comments: { lineComment: ";" },
  brackets: [["[", "]"], ["(", ")"]],
  autoClosingPairs: [{ open: "(", close: ")" }, { open: '"', close: '"' }],
});
monaco.languages.setMonarchTokensProvider("cns", {
  ignoreCase: true,
  tokenizer: {
    root: [
      [/;.*$/, "comment"],
      [/^\s*\[[^\]]*\]/, "keyword"],
      // "key =" at the start of a line (trigger1, type, var(3), ...).
      [/^(\s*)([a-z_][\w.]*(?:\([^)]*\))?)(\s*)(?==)/, ["", "variable", ""]],
      { include: "@value" },
    ],
    value: [
      [/;.*$/, "comment"],
      [/"[^"]*"?/, "string"],
      [/\d*\.\d+(e[+-]?\d+)?|\d+(e[+-]?\d+)?/, "number"],
      [/[a-z_][\w.]*/, "identifier"],
      [/:=|[=!<>]=?|&&|\|\||\^\^|\*\*|[-+*/%&|^~]/, "operator"],
      [/[()[\],]/, "delimiter"],
    ],
  },
} as monaco.languages.IMonarchLanguage);

monaco.languages.register({ id: "zss", extensions: [".zss"], aliases: ["ZSS"] });
monaco.languages.setLanguageConfiguration("zss", {
  comments: { lineComment: "#" },
  brackets: [["{", "}"], ["[", "]"], ["(", ")"]],
  autoClosingPairs: [{ open: "{", close: "}" }, { open: "(", close: ")" }, { open: '"', close: '"' }],
  indentationRules: { increaseIndentPattern: /\{[^}]*$/, decreaseIndentPattern: /^\s*\}/ },
});
monaco.languages.setMonarchTokensProvider("zss", {
  ignoreCase: true,
  keywords: ["if", "else", "switch", "case", "default", "for", "while", "let", "call", "break", "continue", "ignorehitpause", "persistent"],
  tokenizer: {
    root: [
      [/#.*$/, "comment"],
      [/^\s*\[[^\]]*\]/, "keyword"],
      [/"[^"]*"?/, "string"],
      [/\$[a-z_]\w*/, "variable.predefined"],
      // controllerName{ and param: (but not :=)
      [/[a-z_][\w.]*(?=\s*\{)/, "type"],
      [/[a-z_][\w.]*(?=\s*:(?!=))/, "variable"],
      [/[a-z_][\w.]*/, { cases: { "@keywords": "keyword", "@default": "identifier" } }],
      [/\d*\.\d+(e[+-]?\d+)?|\d+(e[+-]?\d+)?/, "number"],
      [/:=|[=!<>]=?|&&|\|\||\^\^|\*\*|[-+*/%&|^~]/, "operator"],
      [/[{}()[\],;:]/, "delimiter"],
    ],
  },
} as monaco.languages.IMonarchLanguage);

// --- Theme ---------------------------------------------------------------------

// defineTheme reads the colors from the CSS tokens in public/style.css, so
// the editor follows the app palette with a single source.
export function defineTheme(): string {
  const css = getComputedStyle(document.documentElement);
  const v = (name: string) => css.getPropertyValue(name).trim();
  const hex = (name: string) => v(name).replace(/^#/, "");
  monaco.editor.defineTheme("katafighter", {
    base: "vs-dark",
    inherit: true,
    rules: [
      { token: "comment", foreground: hex("--faint"), fontStyle: "italic" },
      { token: "keyword", foreground: hex("--accent") },
      { token: "type", foreground: hex("--accent") },
      { token: "variable", foreground: hex("--strike-2") },
      { token: "variable.predefined", foreground: hex("--ok") },
      { token: "string", foreground: hex("--ok") },
      { token: "number", foreground: hex("--warning") },
      { token: "operator", foreground: hex("--muted") },
      { token: "delimiter", foreground: hex("--muted") },
      { token: "identifier", foreground: hex("--text") },
    ],
    colors: {
      "editor.background": v("--editor"),
      "editor.foreground": v("--text"),
      "editorLineNumber.foreground": v("--faint"),
      "editorLineNumber.activeForeground": v("--text"),
      "editor.selectionBackground": v("--selection"),
      "editor.lineHighlightBackground": v("--raised") + "80",
      "editorCursor.foreground": v("--accent"),
      "editorError.foreground": v("--error"),
      "editorWarning.foreground": v("--warning"),
      "editorWidget.background": v("--raised"),
      "editorWidget.border": v("--border"),
      "editorHoverWidget.background": v("--raised"),
      "editorHoverWidget.border": v("--border"),
      "scrollbarSlider.background": v("--hover") + "aa",
    },
  });
  return "katafighter";
}

// --- Documents -----------------------------------------------------------------

export type Doc = {
  path: string;
  model: monaco.editor.ITextModel;
  version: string; // EditorService version of the content on disk
  savedId: number; // model.getAlternativeVersionId() at the last load or save
  readOnly: boolean;
  reason: string;
  view: monaco.editor.ICodeEditorViewState | null;
};

const docs = new Map<string, Doc>();

export function getDoc(path: string): Doc | undefined {
  return docs.get(path);
}

// openDoc loads a file once; later calls return the same document.
export async function openDoc(path: string): Promise<Doc> {
  const have = docs.get(path);
  if (have) return have;
  const f = await EditorService.Open(path);
  if (!f) throw new Error(`could not open ${path}`);
  const again = docs.get(path); // opened by a concurrent call
  if (again) return again;
  const model = monaco.editor.createModel(f.text, f.language, monaco.Uri.file(path));
  const doc: Doc = {
    path,
    model,
    version: f.version,
    savedId: model.getAlternativeVersionId(),
    readOnly: f.readOnly,
    reason: f.reason,
    view: null,
  };
  docs.set(path, doc);
  return doc;
}

export function isDirty(doc: Doc): boolean {
  return doc.model.getAlternativeVersionId() !== doc.savedId;
}

export function dirtyPaths(): string[] {
  return [...docs.values()].filter(isDirty).map((d) => d.path);
}

export async function saveDoc(path: string): Promise<void> {
  const doc = docs.get(path);
  if (!doc || doc.readOnly) return;
  const id = doc.model.getAlternativeVersionId();
  doc.version = await EditorService.Save(path, doc.model.getValue(), doc.version);
  doc.savedId = id;
}

export function closeDoc(path: string) {
  docs.get(path)?.model.dispose();
  docs.delete(path);
}

export function closeAllDocs() {
  for (const p of [...docs.keys()]) closeDoc(p);
}
