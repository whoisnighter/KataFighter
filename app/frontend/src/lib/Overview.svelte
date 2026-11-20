<script lang="ts">
  import FileIcon, { fileIconFor } from "./FileIcon.svelte";
  import type { Character } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  let { character, onopen }: { character: Character; onopen?: (view: "sprites" | "anim") => void } = $props();
  const has = (role: string) => !!character.files?.find((f) => f.role === role)?.path;
</script>

<div class="overview">
  <h1>{character.displayName || character.name}</h1>
  <p class="sub">{character.author ? `by ${character.author}` : "Unknown author"}</p>
  {#if onopen}
    <div class="open">
      <button class="btn" disabled={!has("sprite")} onclick={() => onopen("sprites")}><FileIcon name="sprite" size={14} />Sprites</button>
      <button class="btn" disabled={!has("anim") || !has("sprite")} onclick={() => onopen("anim")}><FileIcon name="anim" size={14} />Animations</button>
    </div>
  {/if}

  <h2>Properties</h2>
  <table class="props">
    <tbody>
      <tr><th>Name</th><td>{character.name}</td></tr>
      <tr><th>Display name</th><td>{character.displayName}</td></tr>
      <tr><th>Local coord</th><td>{character.localCoord[0]} × {character.localCoord[1]}</td></tr>
      <tr><th>Definition</th><td class="mono">{character.defPath}</td></tr>
      <tr><th>Engine folder</th><td class="mono">{character.engineDir || "not found"}</td></tr>
    </tbody>
  </table>

  <h2>Files</h2>
  <table class="files">
    <thead><tr><th>Role</th><th>File</th><th>Contents</th></tr></thead>
    <tbody>
      {#each character.files ?? [] as f}
        <tr class:missing={!f.path} title={f.path || "not found"}>
          <td class="mono muted">{f.role}</td>
          <td><span class="fname"><FileIcon name={fileIconFor(f.role, f.name)} size={14} />{f.name}</span></td>
          <td class="muted">{f.summary}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .overview { padding: 24px 32px 32px; max-width: 960px; }
  h1 { margin: 0; font-size: 22px; font-weight: 600; color: var(--text-strong); }
  .sub { margin: 4px 0 0; color: var(--muted); }
  .open { display: flex; gap: 8px; margin-top: 16px; }
  .open .btn { display: inline-flex; align-items: center; gap: 6px; }
  h2 {
    margin: 28px 0 8px; font-size: 11px; font-weight: 600; letter-spacing: 0.06em;
    text-transform: uppercase; color: var(--muted);
  }
  table { border-collapse: collapse; width: 100%; }
  th, td { text-align: left; padding: 5px 12px 5px 0; vertical-align: top; }
  .props th { width: 140px; font-weight: 400; color: var(--muted); }
  .props td { user-select: text; }
  .files thead th { font-weight: 400; color: var(--muted); border-bottom: 1px solid var(--border); }
  .files tbody tr { border-bottom: 1px solid var(--border); }
  .files tbody tr:hover { background: var(--hover); }
  .files td:first-child { width: 90px; }
  .missing td { color: var(--error); }
  .fname { display: inline-flex; align-items: center; gap: 6px; }
</style>
