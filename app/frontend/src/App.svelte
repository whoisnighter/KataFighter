<script lang="ts">
  import { onMount } from "svelte";
  import { Browser, Window } from "@wailsio/runtime";
  import { ProjectService } from "../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { Character } from "../bindings/github.com/whoisnighter/KataFighter/app/services/models";
  import TitleBar, { type Menu } from "./lib/TitleBar.svelte";
  import Explorer from "./lib/Explorer.svelte";
  import Welcome from "./lib/Welcome.svelte";
  import Overview from "./lib/Overview.svelte";
  import Problems from "./lib/Problems.svelte";
  import StatusBar from "./lib/StatusBar.svelte";
  import QuickOpen from "./lib/QuickOpen.svelte";
  import Icon from "./lib/Icon.svelte";
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

  const desktop = $derived(mode === "desktop");
  const problems = $derived(character?.problems ?? []);
  const errors = $derived(problems.filter((p) => p.severity === "error").length);
  const warnings = $derived(problems.filter((p) => p.severity === "warning").length);
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

  async function openPath(path: string) {
    quickOpen = false;
    error = "";
    loading = true;
    try {
      const c = await ProjectService.Open(path);
      if (c) {
        character = c;
        saveRecent([c.defPath, ...recent.filter((p) => p !== c.defPath)]);
      }
    } catch (e) {
      error = `Could not open ${baseName(path)}: ${e}`;
    } finally {
      loading = false;
    }
  }

  function closeCharacter() {
    character = null;
  }

  const menus: Menu[] = $derived([
    {
      label: "File",
      items: [
        { label: "Open Character…", shortcut: "Ctrl+O", action: open },
        { label: "Close Character", shortcut: "Ctrl+W", action: closeCharacter, disabled: !character },
        { separator: true },
        { label: "Exit", action: () => Window.Close(), disabled: !desktop },
      ],
    },
    {
      label: "View",
      items: [
        { label: showSidebar ? "Hide Explorer" : "Show Explorer", shortcut: "Ctrl+B", action: () => (showSidebar = !showSidebar), disabled: !character },
        { label: showPanel ? "Hide Problems" : "Show Problems", shortcut: "Ctrl+J", action: () => (showPanel = !showPanel), disabled: !character },
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
    else if (k === "w" && character) closeCharacter();
    else if (k === "b" && character) showSidebar = !showSidebar;
    else if (k === "j" && character) showPanel = !showPanel;
    else return;
    e.preventDefault();
  }

  onMount(async () => {
    mode = await ProjectService.Mode().catch(() => "browser");
    // ?def=<path> opens a character directly (used by agents in browser mode).
    const def = new URLSearchParams(location.search).get("def");
    if (def) await openPath(def);
  });
</script>

<svelte:window onkeydown={shortcuts} />

<div class="app">
  <TitleBar {menus} {title} {desktop} />

  <div class="body" class:with-sidebar={character && showSidebar}>
    {#if character && showSidebar}
      <aside><Explorer {character} onclose={closeCharacter} /></aside>
    {/if}

    <main>
      <div class="tabs">
        <div class="tab active">
          {#if character}
            <Icon name="def" size={14} /><span>{character.displayName || character.name}</span>
          {:else}
            <img src={logo} alt="" /><span>Welcome</span>
          {/if}
        </div>
      </div>

      {#if error}
        <div class="banner" role="alert">
          <Icon name="error" size={14} /><span>{error}</span>
          <button class="dismiss" aria-label="Dismiss" onclick={() => (error = "")}><Icon name="close" size={12} /></button>
        </div>
      {/if}

      <div class="content">
        {#if character}
          <Overview {character} />
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
        <div class="panel"><Problems {problems} onclose={() => (showPanel = false)} /></div>
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
      <p class="muted">MIT License. Reads files the way Ikemen GO v1.0.0 does.</p>
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
  .tab :global(.icon) { color: var(--accent); }

  .banner {
    flex: none;
    display: flex; align-items: center; gap: 8px; padding: 6px 12px;
    background: rgba(255, 61, 90, 0.12); border-bottom: 1px solid rgba(255, 61, 90, 0.35); color: var(--text);
  }
  .banner :global(.icon) { color: var(--error); }
  .dismiss { margin-left: auto; display: grid; place-items: center; width: 20px; height: 20px; border-radius: var(--radius); color: var(--muted); }
  .dismiss:hover { background: var(--hover); color: var(--text); }

  .content { flex: 1; min-height: 0; overflow: auto; }
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
