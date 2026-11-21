<script lang="ts">
  import { onMount } from "svelte";
  import { Browser, Window } from "@wailsio/runtime";
  import { ProjectService } from "../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { Character, Problem } from "../bindings/github.com/whoisnighter/KataFighter/app/services/models";
  import TitleBar, { type Menu } from "./lib/TitleBar.svelte";
  import Explorer from "./lib/Explorer.svelte";
  import Welcome from "./lib/Welcome.svelte";
  import Overview from "./lib/Overview.svelte";
  import SpriteBrowser from "./lib/SpriteBrowser.svelte";
  import AnimPlayer from "./lib/AnimPlayer.svelte";
  import CodeEditor, { type Reveal } from "./lib/CodeEditor.svelte";
  import { saveDoc, closeDoc, closeAllDocs, dirtyPaths } from "./lib/monaco";
  import type { ActFile, PaletteChoice } from "./lib/sprites";
  import Problems from "./lib/Problems.svelte";
  import StatusBar from "./lib/StatusBar.svelte";
  import QuickOpen from "./lib/QuickOpen.svelte";
  import Icon from "./lib/Icon.svelte";
  import FileIcon, { fileIconFor, type FileIconName } from "./lib/FileIcon.svelte";
  import logo from "../../../docs/assets/logo.svg";

  const REPO = "https://github.com/whoisnighter/KataFighter";
  const RECENT_KEY = "katafighter.recent";

  let mode = $state("desktop");
  let character = $state<Character | null>(null);
  let loading = $state(false);
  let error = $state("");
  let showSidebar = $state(true);
  let showPanel = $state(true);
  let quickOpen = $state(false);
  let about = $state(false);
  let recent = $state<string[]>(loadRecent());

  // Editor tabs. The overview is always open; the viewers and the code
  // editor ("file:" + path) open on demand.
  type ViewId = "overview" | "sprites" | "anim";
  type TabId = ViewId | `file:${string}`;
  let tabs = $state<TabId[]>(["overview"]);
  let active = $state<TabId>("overview");
  let palette = $state<PaletteChoice>(""); // shared by the sprite browser and the animation player
  let dirty = $state<Record<string, boolean>>({}); // unsaved files, by path
  // Problems of the files open in the code editor, checked as you type.
  // They replace the problems found when the character was opened.
  let live = $state<Record<string, Problem[]>>({});
  let reveal = $state<Reveal | null>(null);

  const desktop = $derived(mode === "desktop");
  const activeFile = $derived(active.startsWith("file:") ? active.slice(5) : "");
  const problems = $derived([
    ...(character?.problems ?? []).filter((p) => !(p.file in live)),
    ...Object.values(live).flat(),
  ]);
  const errors = $derived(problems.filter((p) => p.severity === "error").length);
  const warnings = $derived(problems.filter((p) => p.severity === "warning").length);
  const spritePath = $derived(character?.files?.find((f) => f.role === "sprite")?.path ?? "");
  // The DEF's pal1..palN files that were found (ACT palettes).
  const acts = $derived<ActFile[]>(
    (character?.files ?? []).flatMap((f) => {
      const m = f.role.match(/^pal(\d+)$/);
      return m && f.path ? [{ slot: Number(m[1]), name: f.name, path: f.path }] : [];
    }),
  );
  const animPath = $derived(character?.files?.find((f) => f.role === "anim")?.path ?? "");
  const title = $derived(character ? `${baseName(character.defPath)} — KataFighter` : "KataFighter");

  $effect(() => {
    document.title = title;
    if (desktop) Window.SetTitle(title).catch(() => {});
  });

  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }

  function loadRecent(): string[] {
    try {
      return JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    } catch {
      return [];
    }
  }
  function saveRecent(list: string[]) {
    recent = list.slice(0, 10);
    try {
      localStorage.setItem(RECENT_KEY, JSON.stringify(recent));
    } catch {}
  }

  async function open() {
    error = "";
    if (!desktop) {
      quickOpen = true;
      return;
    }
    try {
      const path = await ProjectService.ChooseDef();
      if (path) await openPath(path);
    } catch (e) {
      error = String(e);
    }
  }

  // confirmDiscard asks before unsaved changes in paths are lost.
  function confirmDiscard(paths: string[]): boolean {
    const unsaved = paths.filter((p) => dirty[p]);
    if (unsaved.length === 0) return true;
    return confirm(`Discard unsaved changes in ${unsaved.map(baseName).join(", ")}?`);
  }

  function resetEditors() {
    closeAllDocs();
    dirty = {};
    live = {};
    reveal = null;
  }

  async function openPath(path: string) {
    if (!confirmDiscard(dirtyPaths())) return;
    quickOpen = false;
    error = "";
    loading = true;
    try {
      const c = await ProjectService.Open(path);
      if (c) {
        resetEditors();
        character = c;
        tabs = ["overview"];
        active = "overview";
        palette = "";
        saveRecent([c.defPath, ...recent.filter((p) => p !== c.defPath)]);
      }
    } catch (e) {
      error = `Could not open ${baseName(path)}: ${e}`;
    } finally {
      loading = false;
    }
  }

  function closeCharacter() {
    if (!confirmDiscard(dirtyPaths())) return;
    resetEditors();
    character = null;
  }

  // isCode reports whether a file opens in the code editor.
  function isCode(path: string): boolean {
    return /\.(cns|st|cmd|zss|const|def)$/i.test(path);
  }

  function openFile(path: string, line = 0, col = 0) {
    if (!character || !path) return;
    openTab(`file:${path}`);
    if (line > 0) reveal = { path, line, col, seq: (reveal?.seq ?? 0) + 1 };
  }

  async function save() {
    const path = activeFile;
    if (!path || !dirty[path]) return;
    try {
      await saveDoc(path);
      dirty = { ...dirty, [path]: false };
    } catch (e) {
      error = `Could not save ${baseName(path)}: ${e}`;
    }
  }

  function selectProblem(p: Problem) {
    if (isCode(p.file)) openFile(p.file, p.line, p.col);
  }

  function canOpen(id: TabId): boolean {
    if (id.startsWith("file:")) return !!character;
    if (id === "sprites") return !!spritePath;
    if (id === "anim") return !!animPath && !!spritePath;
    return !!character;
  }
  function openTab(id: TabId) {
    if (!canOpen(id)) return;
    if (!tabs.includes(id)) tabs = [...tabs, id];
    active = id;
  }
  function closeTab(id: TabId) {
    if (id.startsWith("file:")) {
      const path = id.slice(5);
      if (!confirmDiscard([path])) return;
      closeDoc(path);
      const { [path]: _d, ...restDirty } = dirty;
      const { [path]: _l, ...restLive } = live;
      dirty = restDirty;
      live = restLive;
    }
    const i = tabs.indexOf(id);
    tabs = tabs.filter((t) => t !== id);
    if (active === id) active = tabs[Math.max(0, i - 1)];
  }
  function tabInfo(id: TabId): { label: string; icon: FileIconName } {
    if (id.startsWith("file:")) {
      const name = baseName(id.slice(5));
      return { label: dirty[id.slice(5)] ? `● ${name}` : name, icon: fileIconFor("", name) };
    }
    if (id === "sprites") return { label: `Sprites · ${baseName(spritePath)}`, icon: "sprite" };
    if (id === "anim") return { label: `Animations · ${baseName(animPath)}`, icon: "anim" };
    return { label: character?.displayName || character?.name || "", icon: "def" };
  }

  const menus: Menu[] = $derived([
    {
      label: "File",
      items: [
        { label: "Open Character…", shortcut: "Ctrl+O", action: open },
        { label: "Save", shortcut: "Ctrl+S", action: save, disabled: !activeFile || !dirty[activeFile] },
        { label: "Close Editor", shortcut: "Ctrl+W", action: () => closeTab(active), disabled: !activeFile },
        { label: "Close Character", action: closeCharacter, disabled: !character },
        { separator: true },
        { label: "Exit", action: () => Window.Close(), disabled: !desktop },
      ],
    },
    {
      label: "View",
      items: [
        { label: showSidebar ? "Hide Explorer" : "Show Explorer", shortcut: "Ctrl+B", action: () => (showSidebar = !showSidebar), disabled: !character },
        { label: showPanel ? "Hide Problems" : "Show Problems", shortcut: "Ctrl+J", action: () => (showPanel = !showPanel), disabled: !character },
        { separator: true },
        { label: "Overview", shortcut: "Ctrl+1", action: () => openTab("overview"), disabled: !character },
        { label: "Sprites", shortcut: "Ctrl+2", action: () => openTab("sprites"), disabled: !character || !canOpen("sprites") },
        { label: "Animations", shortcut: "Ctrl+3", action: () => openTab("anim"), disabled: !character || !canOpen("anim") },
      ],
    },
    {
      label: "Help",
      items: [
        { label: "KataFighter on GitHub", action: () => Browser.OpenURL(REPO).catch(() => window.open(REPO)) },
        { separator: true },
        { label: "About KataFighter", action: () => (about = true) },
      ],
    },
  ]);

  function shortcuts(e: KeyboardEvent) {
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
    const k = e.key.toLowerCase();
    if (k === "o") open();
    else if (k === "s") save();
    else if (k === "w" && activeFile) closeTab(active);
    else if (k === "w" && character) closeCharacter();
    else if (k === "b" && character) showSidebar = !showSidebar;
    else if (k === "j" && character) showPanel = !showPanel;
    else if (k === "1" && character) openTab("overview");
    else if (k === "2" && character) openTab("sprites");
    else if (k === "3" && character) openTab("anim");
    else return;
    e.preventDefault();
  }

  onMount(async () => {
    mode = await ProjectService.Mode().catch(() => "browser");
    // ?def=<path> opens a character directly (used by agents in browser mode).
    const def = new URLSearchParams(location.search).get("def");
    if (def) await openPath(def);
    // &view=sprites|anim opens a viewer tab.
    const view = new URLSearchParams(location.search).get("view");
    if (view === "sprites" || view === "anim") openTab(view);
    // &file=<path>[&line=N] opens a file in the code editor.
    const file = new URLSearchParams(location.search).get("file");
    if (file) openFile(file, Number(new URLSearchParams(location.search).get("line")) || 0);
  });

  // Leaving the page with unsaved files asks first (browser mode).
  function beforeUnload(e: BeforeUnloadEvent) {
    if (Object.values(dirty).some(Boolean)) e.preventDefault();
  }
</script>

<svelte:window onkeydown={shortcuts} onbeforeunload={beforeUnload} />

<div class="app">
  <TitleBar {menus} {title} {desktop} />

  <div class="body" class:with-sidebar={character && showSidebar}>
    {#if character && showSidebar}
      <aside><Explorer
          {character}
          {activeFile}
          onclose={closeCharacter}
          onopen={(role) => openTab(role === "sprite" ? "sprites" : role === "anim" ? "anim" : "overview")}
          onopenfile={(path) => openFile(path)}
        /></aside>
    {/if}

    <main>
      <div class="tabs">
        {#if character}
          {#each tabs as id (id)}
            {@const t = tabInfo(id)}
            <div class="tab" class:active={active === id}>
              <button class="tab-open" onclick={() => (active = id)}><FileIcon name={t.icon} size={14} /><span>{t.label}</span></button>
              {#if id !== "overview"}
                <button class="tab-close" aria-label="Close {t.label}" onclick={() => closeTab(id)}><Icon name="close" size={12} /></button>
              {/if}
            </div>
          {/each}
        {:else}
          <div class="tab active"><img src={logo} alt="" /><span>Welcome</span></div>
        {/if}
      </div>

      {#if error}
        <div class="banner" role="alert">
          <Icon name="error" size={14} /><span>{error}</span>
          <button class="dismiss" aria-label="Dismiss" onclick={() => (error = "")}><Icon name="close" size={12} /></button>
        </div>
      {/if}

      <div class="content" class:fill={character && active !== "overview"}>
        {#if character && activeFile}
          <CodeEditor
            path={activeFile}
            {reveal}
            onproblems={(path, ps) => (live = { ...live, [path]: ps })}
            ondirty={(path, d) => dirty[path] !== d && (dirty = { ...dirty, [path]: d })}
            onerror={(m) => (error = m)}
          />
        {:else if character && active === "sprites"}
          <SpriteBrowser path={spritePath} {acts} bind:palette />
        {:else if character && active === "anim"}
          <AnimPlayer {animPath} {spritePath} {acts} bind:palette />
        {:else if character}
          <Overview {character} onopen={openTab} />
        {:else}
          <Welcome
            {recent}
            onopen={open}
            onopenpath={openPath}
            onforget={(p) => saveRecent(recent.filter((r) => r !== p))}
          />
        {/if}
      </div>

      {#if character && showPanel}
        <div class="panel"><Problems {problems} onselect={selectProblem} onclose={() => (showPanel = false)} /></div>
      {/if}
    </main>
  </div>

  <StatusBar
    {errors}
    {warnings}
    {mode}
    {loading}
    localCoord={character ? [character.localCoord[0], character.localCoord[1]] : null}
    ontoggleproblems={() => character && (showPanel = !showPanel)}
  />
</div>

{#if quickOpen}
  <QuickOpen initial={recent[0] ?? ""} onsubmit={openPath} oncancel={() => (quickOpen = false)} />
{/if}

{#if about}
  <div class="backdrop" role="presentation" onmousedown={(e) => e.target === e.currentTarget && (about = false)}>
    <div class="about" role="dialog" aria-label="About KataFighter">
      <img src={logo} alt="" />
      <h2>KataFighter</h2>
      <p class="muted">Character editor for Ikemen GO · version 0.1.0</p>
      <p class="muted">MIT License</p>
      <button class="btn primary" onclick={() => (about = false)}>OK</button>
    </div>
  </div>
{/if}

<style>
  .app { display: flex; flex-direction: column; height: 100%; }
  .body { flex: 1; display: grid; grid-template-columns: 1fr; min-height: 0; }
  .body.with-sidebar { grid-template-columns: 260px 1fr; }
  aside { min-width: 0; border-right: 1px solid var(--border); }

  main { display: flex; flex-direction: column; min-width: 0; min-height: 0; background: var(--editor); }

  .tabs { display: flex; flex: none; height: 35px; background: var(--sidebar); border-bottom: 1px solid var(--border); }
  .tab {
    display: flex; align-items: center; gap: 6px; padding: 0 14px; height: 100%;
    color: var(--muted); border-right: 1px solid var(--border);
  }
  .tab.active { background: var(--editor); color: var(--text); box-shadow: inset 0 1px 0 var(--accent); margin-bottom: -1px; }
  .tab img { width: 14px; height: 14px; }
  .tab-open { display: flex; align-items: center; gap: 6px; height: 100%; white-space: nowrap; }
  .tab:has(.tab-close) { padding-right: 6px; }
  .tab-close { display: grid; place-items: center; width: 20px; height: 20px; border-radius: var(--radius); color: var(--muted); visibility: hidden; }
  .tab.active .tab-close, .tab:hover .tab-close { visibility: visible; }
  .tab-close:hover { background: var(--hover); color: var(--text); }

  .banner {
    flex: none;
    display: flex; align-items: center; gap: 8px; padding: 6px 12px;
    background: rgba(255, 61, 90, 0.12); border-bottom: 1px solid rgba(255, 61, 90, 0.35); color: var(--text);
  }
  .banner :global(.icon) { color: var(--error); }
  .dismiss { margin-left: auto; display: grid; place-items: center; width: 20px; height: 20px; border-radius: var(--radius); color: var(--muted); }
  .dismiss:hover { background: var(--hover); color: var(--text); }

  .content { flex: 1; min-height: 0; overflow: auto; }
  .content.fill { overflow: hidden; }
  .panel { flex: 0 0 200px; min-height: 0; }

  .backdrop { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; background: rgba(0, 0, 0, 0.35); }
  .about {
    width: 360px; padding: 24px; text-align: center;
    background: var(--raised); border: 1px solid var(--border); border-radius: 8px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
  }
  .about img { width: 64px; height: 64px; }
  .about h2 { margin: 8px 0 4px; font-size: 18px; color: var(--text-strong); }
  .about p { margin: 4px 0; }
  .about .btn { margin-top: 16px; min-width: 80px; }
</style>
