<script lang="ts" module>
  // Reveal asks the editor to jump to a line (from the Problems panel).
  // seq changes on every request, so the same line can be revealed twice.
  export type Reveal = { path: string; line: number; col: number; seq: number };

  // The last reveal applied, shared by editor instances: the editor is
  // recreated when its tab comes back, and an old request must not move the
  // cursor again.
  let revealed = 0;
</script>

<script lang="ts">
  import { untrack } from "svelte";
  import { monaco, defineTheme, openDoc, isDirty, type Doc } from "./monaco";
  import Icon from "./Icon.svelte";
  import { EditorService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { Problem } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  let {
    path,
    reveal = null,
    onproblems,
    ondirty,
    onerror,
  }: {
    path: string;
    reveal?: Reveal | null;
    onproblems: (path: string, problems: Problem[]) => void;
    ondirty: (path: string, dirty: boolean) => void;
    onerror: (message: string) => void;
  } = $props();

  let host: HTMLDivElement;
  let editor: monaco.editor.IStandaloneCodeEditor | null = null;
  let doc = $state.raw<Doc | null>(null);
  let sub: monaco.IDisposable | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const CHECK_DELAY = 250; // ms after the last keystroke

  function ensureEditor(): monaco.editor.IStandaloneCodeEditor {
    if (editor) return editor;
    const mono = getComputedStyle(document.documentElement).getPropertyValue("--mono").trim();
    editor = monaco.editor.create(host, {
      model: null,
      theme: defineTheme(),
      automaticLayout: true,
      fontFamily: mono,
      fontSize: 13,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      renderWhitespace: "selection",
      fixedOverflowWidgets: true,
    });
    return editor;
  }

  // Load when path changes, and only then: load reads doc, which must not
  // become a dependency (setting doc would reload and undo a reveal).
  $effect(() => {
    const p = path;
    untrack(() => load(p));
  });

  $effect(() => {
    if (reveal && doc) applyReveal(reveal);
  });

  $effect(() => () => {
    clearTimeout(timer);
    if (doc && editor) doc.view = editor.saveViewState();
    sub?.dispose();
    editor?.dispose();
    editor = null;
  });

  async function load(p: string) {
    const ed = ensureEditor();
    if (doc && doc.path !== p) doc.view = ed.saveViewState();
    let d: Doc;
    try {
      d = await openDoc(p);
    } catch (e) {
      onerror(`Could not open ${p}: ${e}`);
      return;
    }
    if (p !== path || editor !== ed) return; // another file was asked for meanwhile
    sub?.dispose();
    clearTimeout(timer);
    ed.setModel(d.model);
    ed.updateOptions({ readOnly: d.readOnly });
    if (d.view) ed.restoreViewState(d.view);
    ed.focus();
    // doc is set last: the reveal effect waits for it, so a reveal that came
    // with the request to open this file wins over the restored position.
    doc = d;
    sub = d.model.onDidChangeContent(() => {
      ondirty(d.path, isDirty(d));
      clearTimeout(timer);
      timer = setTimeout(() => check(d), CHECK_DELAY);
    });
    check(d);
  }

  function severity(s: string): monaco.MarkerSeverity {
    if (s === "error") return monaco.MarkerSeverity.Error;
    if (s === "warning") return monaco.MarkerSeverity.Warning;
    if (s === "hint") return monaco.MarkerSeverity.Hint;
    return monaco.MarkerSeverity.Info;
  }

  async function check(d: Doc) {
    if (d.model.isDisposed()) return;
    const id = d.model.getVersionId();
    let ms;
    try {
      ms = (await EditorService.Check(d.path, d.model.getValue())) ?? [];
    } catch (e) {
      onerror(`Could not check ${d.path}: ${e}`);
      return;
    }
    if (d.model.isDisposed() || d.model.getVersionId() !== id) return; // stale
    monaco.editor.setModelMarkers(
      d.model,
      "katafighter",
      ms.map((m) => ({
        startLineNumber: m.startLine,
        startColumn: m.startCol,
        endLineNumber: m.endLine,
        endColumn: m.endLine === m.startLine && m.endCol <= m.startCol ? m.startCol + 1 : m.endCol,
        severity: severity(m.severity),
        message: m.message,
        code: m.code || undefined,
        source: "katafighter",
      })),
    );
    onproblems(
      d.path,
      ms.map((m) => ({ file: d.path, line: m.line, col: m.col, severity: m.severity, message: m.message })),
    );
  }

  function applyReveal(r: Reveal) {
    if (!editor || !doc || r.path !== doc.path || r.seq <= revealed) return;
    revealed = r.seq;
    const line = Math.max(1, r.line);
    editor.revealLineInCenter(line);
    editor.setPosition({ lineNumber: line, column: Math.max(1, r.col) });
    editor.focus();
  }
</script>

<div class="code">
  {#if doc?.readOnly}
    <div class="notice"><Icon name="info" size={14} /><span>{doc.reason}</span></div>
  {/if}
  <div class="host" bind:this={host}></div>
</div>

<style>
  .code { display: flex; flex-direction: column; height: 100%; }
  .notice {
    flex: none; display: flex; align-items: center; gap: 8px; padding: 6px 12px;
    background: var(--raised); border-bottom: 1px solid var(--border); color: var(--muted);
  }
  .host { flex: 1; min-height: 0; }
</style>
