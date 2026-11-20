<script lang="ts">
  import Icon from "./Icon.svelte";

  let { errors, warnings, localCoord, mode, loading, ontoggleproblems }: {
    errors: number;
    warnings: number;
    localCoord: [number, number] | null;
    mode: string;
    loading: boolean;
    ontoggleproblems: () => void;
  } = $props();
</script>

<footer class="status">
  <button class="item" title="Toggle Problems (Ctrl+J)" onclick={ontoggleproblems}>
    <span class:err={errors > 0}><Icon name="error" size={14} /></span>{errors}
    <span class:warn={warnings > 0}><Icon name="warning" size={14} /></span>{warnings}
  </button>
  {#if loading}<span class="item">Loading…</span>{/if}
  <span class="spacer"></span>
  {#if localCoord}<span class="item">localcoord {localCoord[0]}×{localCoord[1]}</span>{/if}
  {#if mode === "browser"}<span class="item badge">Browser mode</span>{/if}
  <span class="item">Ikemen GO v1.0.0</span>
</footer>

<style>
  .status {
    display: flex; align-items: center; height: 22px; flex: none;
    background: var(--statusbar); border-top: 1px solid var(--border);
    font-size: var(--small); color: var(--muted);
  }
  .item { display: flex; align-items: center; gap: 4px; height: 100%; padding: 0 8px; color: inherit; }
  button.item:hover { background: var(--hover); color: var(--text); }
  .err { color: var(--error); display: grid; }
  .warn { color: var(--warning); display: grid; }
  span:has(> :global(.icon)) { display: grid; }
  .spacer { flex: 1; }
  .badge { color: var(--accent); }
</style>
