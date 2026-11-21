<script lang="ts">
  import Icon from "./Icon.svelte";
  import type { Problem } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  let {
    problems,
    onselect,
    onclose,
  }: { problems: Problem[]; onselect?: (p: Problem) => void; onclose: () => void } = $props();

  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }
</script>

<div class="panel">
  <div class="tabs">
    <span class="tab active">Problems <span class="badge">{problems.length}</span></span>
    <button class="action" title="Close panel (Ctrl+J)" onclick={onclose}><Icon name="close" size={14} /></button>
  </div>
  <div class="list">
    {#if problems.length === 0}
      <p class="empty">No problems have been detected in this character.</p>
    {:else}
      {#each problems as p}
        <button class="row {p.severity}" title={p.file} onclick={() => onselect?.(p)}>
          <span class="sev"><Icon name={p.severity === "error" ? "error" : p.severity === "warning" ? "warning" : "info"} /></span>
          <span class="msg">{p.message}</span>
          <span class="loc">{baseName(p.file)}{p.line ? ` [Ln ${p.line}${p.col ? `, Col ${p.col}` : ""}]` : ""}</span>
        </button>
      {/each}
    {/if}
  </div>
</div>

<style>
  .panel { display: flex; flex-direction: column; height: 100%; background: var(--panel); border-top: 1px solid var(--border); }
  .tabs { display: flex; align-items: center; height: 35px; padding: 0 8px 0 16px; flex: none; }
  .tab {
    font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--muted); padding: 8px 0; border-bottom: 1px solid transparent;
  }
  .tab.active { color: var(--text); border-bottom-color: var(--accent); }
  .badge {
    display: inline-block; min-width: 18px; padding: 0 5px; margin-left: 4px; border-radius: 9px;
    background: var(--raised); color: var(--text); font-size: 11px; text-align: center; letter-spacing: 0;
  }
  .action { margin-left: auto; display: grid; place-items: center; width: 22px; height: 22px; border-radius: var(--radius); color: var(--muted); }
  .action:hover { background: var(--hover); color: var(--text); }
  .list { flex: 1; overflow: auto; padding-bottom: 4px; }
  .empty { margin: 4px 16px; color: var(--muted); }
  .row { display: flex; align-items: center; gap: 8px; width: 100%; height: 22px; padding: 0 16px; text-align: left; }
  .row:hover { background: var(--hover); }
  .sev { display: grid; color: var(--muted); }
  .error .sev { color: var(--error); }
  .warning .sev { color: var(--warning); }
  .msg { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .loc { color: var(--faint); white-space: nowrap; }
</style>
