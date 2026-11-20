<script lang="ts">
  import { ProjectService } from "../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { Character } from "../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  let character = $state<Character | null>(null);
  let error = $state("");
  let loading = $state(false);

  const problems = $derived(character?.problems ?? []);
  const errors = $derived(problems.filter((p) => p.severity === "error").length);
  const warnings = $derived(problems.length - errors);

  async function openCharacter() {
    error = "";
    try {
      const path = await ProjectService.ChooseDef();
      if (!path) return;
      loading = true;
      character = await ProjectService.Open(path);
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  }

  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }
</script>

<div class="shell">
  <header>
    <span class="brand">KataFighter</span>
    <button class="primary" onclick={openCharacter} disabled={loading}>
      {loading ? "Opening…" : "Open character…"}
    </button>
    {#if character}
      <span class="title">{character.displayName || character.name}</span>
      <span class="muted">{baseName(character.defPath)}</span>
    {/if}
  </header>

  {#if error}
    <p class="banner">{error}</p>
  {/if}

  {#if !character}
    <main class="empty">
      <h1>Open a character to start</h1>
      <p class="muted">Pick its <code>.def</code> file. KataFighter reads the sprites, animations and states it references.</p>
      <button class="primary" onclick={openCharacter} disabled={loading}>Open character…</button>
    </main>
  {:else}
    <main class="split">
      <aside>
        <h2>Files</h2>
        <ul class="files">
          {#each character.files ?? [] as f}
            <li class:missing={!f.path} title={f.path || "not found"}>
              <span class="role">{f.role}</span>
              <span class="name">{f.name}</span>
              <span class="summary">{f.summary}</span>
            </li>
          {/each}
        </ul>
      </aside>
      <section>
        <h2>Character</h2>
        <dl>
          <dt>Name</dt><dd>{character.name}</dd>
          <dt>Author</dt><dd>{character.author || "—"}</dd>
          <dt>Local coord</dt><dd>{character.localCoord[0]} × {character.localCoord[1]}</dd>
          <dt>Engine folder</dt><dd class="mono">{character.engineDir || "not found"}</dd>
        </dl>

        <h2>
          Problems
          <span class="count" class:bad={errors > 0}>{errors} errors</span>
          <span class="count" class:warn={warnings > 0}>{warnings} warnings</span>
        </h2>
        {#if problems.length === 0}
          <p class="ok">No problems found.</p>
        {:else}
          <ul class="problems">
            {#each problems as p}
              <li class={p.severity}>
                <span class="mono">{baseName(p.file)}{p.line ? `:${p.line}` : ""}{p.col ? `:${p.col}` : ""}</span>
                {p.message}
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    </main>
  {/if}
</div>

<style>
  .shell { display: flex; flex-direction: column; height: 100%; }
  header {
    display: flex; align-items: center; gap: 12px;
    padding: 8px 12px; background: var(--panel); border-bottom: 1px solid var(--border);
  }
  .brand { font-weight: 700; letter-spacing: 0.02em; color: var(--accent); }
  .title { font-weight: 600; }
  .muted { color: var(--muted); }
  .mono, code { font-family: var(--mono); font-size: 12px; }
  .banner { margin: 0; padding: 8px 12px; background: #3a1f22; color: var(--error); }

  .empty { flex: 1; display: grid; place-content: center; text-align: center; gap: 8px; }
  .empty h1 { margin: 0; font-size: 20px; }
  .empty button { justify-self: center; }

  .split { flex: 1; display: grid; grid-template-columns: minmax(280px, 380px) 1fr; min-height: 0; }
  aside { border-right: 1px solid var(--border); overflow: auto; background: var(--panel); }
  section { overflow: auto; padding: 0 16px 16px; }
  h2 { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); margin: 16px 12px 8px; display: flex; gap: 8px; align-items: center; }
  section h2 { margin-left: 0; }

  .files { list-style: none; margin: 0; padding: 0; }
  .files li { display: grid; grid-template-columns: 72px 1fr; gap: 0 8px; padding: 6px 12px; border-top: 1px solid var(--border); }
  .files .role { color: var(--muted); font-family: var(--mono); font-size: 12px; }
  .files .summary { grid-column: 2; color: var(--muted); font-size: 12px; }
  .files li.missing .name, .files li.missing .summary { color: var(--error); }

  dl { display: grid; grid-template-columns: 120px 1fr; gap: 6px 12px; margin: 0; }
  dt { color: var(--muted); }
  dd { margin: 0; }

  .count { font-size: 11px; padding: 1px 6px; border-radius: 10px; background: var(--panel-2); text-transform: none; letter-spacing: 0; }
  .count.bad { color: var(--error); }
  .count.warn { color: var(--warning); }
  .ok { color: var(--ok); }
  .problems { list-style: none; margin: 0; padding: 0; }
  .problems li { padding: 4px 0; border-top: 1px solid var(--border); }
  .problems li.error .mono { color: var(--error); }
  .problems li.warning .mono { color: var(--warning); }
</style>
