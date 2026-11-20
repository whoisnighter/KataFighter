<script lang="ts">
  import Icon from "./Icon.svelte";
  import logo from "../../../../docs/assets/logo.svg";

  let { recent, onopen, onopenpath, onforget }: {
    recent: string[];
    onopen: () => void;
    onopenpath: (path: string) => void;
    onforget: (path: string) => void;
  } = $props();

  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }
  function dirName(p: string): string {
    const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
    return i > 0 ? p.slice(0, i) : "";
  }
</script>

<div class="welcome">
  <div class="inner">
    <div class="hero">
      <img src={logo} alt="" />
      <div>
        <h1>KataFighter</h1>
        <p>Character editor for Ikemen GO</p>
      </div>
    </div>

    <div class="cols">
      <section>
        <h2>Start</h2>
        <button class="link" onclick={onopen}>
          <Icon name="open" />
          <span>Open Character…</span>
          <kbd>Ctrl+O</kbd>
        </button>
      </section>

      <section>
        <h2>Recent</h2>
        {#if recent.length === 0}
          <p class="muted">Characters you open appear here.</p>
        {:else}
          {#each recent as p}
            <div class="recent">
              <button class="link" title={p} onclick={() => onopenpath(p)}>
                <span>{baseName(p)}</span>
                <span class="dir">{dirName(p)}</span>
              </button>
              <button class="forget" title="Remove from list" onclick={() => onforget(p)}><Icon name="close" size={12} /></button>
            </div>
          {/each}
        {/if}
      </section>
    </div>
  </div>
</div>

<style>
  .welcome { height: 100%; overflow: auto; display: flex; justify-content: center; }
  .inner { width: min(880px, 100%); padding: 72px 48px; }

  .hero { display: flex; align-items: center; gap: 20px; margin-bottom: 48px; }
  .hero img { width: 72px; height: 72px; }
  h1 { margin: 0; font-size: 30px; font-weight: 400; color: var(--text-strong); }
  .hero p { margin: 4px 0 0; color: var(--muted); font-size: 16px; }

  .cols { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; }
  h2 { margin: 0 0 10px; font-size: 16px; font-weight: 400; color: var(--text-strong); }

  .link { display: flex; align-items: center; gap: 8px; padding: 3px 0; color: var(--accent); max-width: 100%; }
  .link:hover span:first-of-type { text-decoration: underline; }
  kbd { font-family: inherit; font-size: 11px; color: var(--muted); margin-left: 8px; }

  .recent { display: flex; align-items: center; gap: 6px; }
  .recent .link { flex: 1; min-width: 0; }
  .dir { color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; text-align: left; }
  .forget { visibility: hidden; color: var(--muted); display: grid; place-items: center; width: 20px; height: 20px; border-radius: var(--radius); }
  .recent:hover .forget { visibility: visible; }
  .forget:hover { background: var(--hover); color: var(--text); }
</style>
