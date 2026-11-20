<script lang="ts">
  let { onsubmit, oncancel, initial = "" }: {
    onsubmit: (path: string) => void;
    oncancel: () => void;
    initial?: string;
  } = $props();

  // The box starts from `initial` once; later edits are the user's.
  // svelte-ignore state_referenced_locally
  let value = $state(initial);
  let input: HTMLInputElement;

  $effect(() => {
    input.focus();
    input.select();
  });

  function key(e: KeyboardEvent) {
    if (e.key === "Enter" && value.trim()) onsubmit(value.trim());
    if (e.key === "Escape") oncancel();
  }
</script>

<div class="backdrop" role="presentation" onmousedown={(e) => e.target === e.currentTarget && oncancel()}>
  <div class="box" role="dialog" aria-label="Open character">
    <input
      bind:this={input}
      bind:value
      onkeydown={key}
      spellcheck="false"
      placeholder="Path to a character .def file"
      data-testid="open-path"
    />
    <p class="hint">Press Enter to open, Escape to cancel. In browser mode, type or paste the full path.</p>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; z-index: 50; display: flex; justify-content: center; align-items: flex-start; }
  .box {
    margin-top: 40px; width: min(600px, 90vw); padding: 6px;
    background: var(--raised); border: 1px solid var(--border); border-radius: 6px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
  }
  input {
    width: 100%; height: 28px; padding: 0 8px;
    font: inherit; font-family: var(--mono); font-size: var(--small);
    color: var(--text); background: var(--editor);
    border: 1px solid var(--accent); border-radius: var(--radius); outline: none;
  }
  .hint { margin: 6px 4px 2px; color: var(--muted); font-size: var(--small); }
</style>
