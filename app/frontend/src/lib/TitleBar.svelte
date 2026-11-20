<script lang="ts" module>
  export type MenuItem =
    | { label: string; shortcut?: string; action: () => void; disabled?: boolean }
    | { separator: true };
  export type Menu = { label: string; items: MenuItem[] };
</script>

<script lang="ts">
  import { Window } from "@wailsio/runtime";
  import Icon from "./Icon.svelte";
  import logo from "../../../../docs/assets/logo.svg";

  let { menus, title, desktop }: { menus: Menu[]; title: string; desktop: boolean } = $props();

  let open = $state<number | null>(null);
  let maximised = $state(false);

  function toggle(i: number) {
    open = open === i ? null : i;
  }
  function run(item: MenuItem) {
    open = null;
    if ("action" in item && !item.disabled) item.action();
  }
  async function toggleMaximise() {
    await Window.ToggleMaximise();
    maximised = await Window.IsMaximised();
  }
</script>

<svelte:window
  onkeydown={(e) => e.key === "Escape" && (open = null)}
  onmousedown={(e) => !(e.target as Element).closest?.(".menubar") && (open = null)}
/>

<!-- Double-click on the empty title bar maximises, like a native one. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<header class="titlebar" ondblclick={(e) => desktop && !(e.target as Element).closest("button") && toggleMaximise()}>
  <img class="logo" src={logo} alt="" />
  <nav class="menubar">
    {#each menus as menu, i}
      <div class="menu">
        <button
          class="top"
          class:active={open === i}
          onclick={() => toggle(i)}
          onmouseenter={() => open !== null && (open = i)}
        >{menu.label}</button>
        {#if open === i}
          <div class="dropdown" role="menu">
            {#each menu.items as item}
              {#if "separator" in item}
                <div class="sep"></div>
              {:else}
                <button class="item" role="menuitem" disabled={item.disabled} onclick={() => run(item)}>
                  <span>{item.label}</span>
                  {#if item.shortcut}<span class="shortcut">{item.shortcut}</span>{/if}
                </button>
              {/if}
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </nav>
  <div class="title">{title}</div>
  {#if desktop}
    <div class="controls">
      <button aria-label="Minimise" onclick={() => Window.Minimise()}><Icon name="minimise" /></button>
      <button aria-label={maximised ? "Restore" : "Maximise"} onclick={toggleMaximise}>
        <Icon name={maximised ? "restore" : "maximise"} size={14} />
      </button>
      <button class="close" aria-label="Close" onclick={() => Window.Close()}><Icon name="close" /></button>
    </div>
  {/if}
</header>

<style>
  .titlebar {
    --wails-draggable: drag;
    position: relative;
    display: flex;
    align-items: center;
    height: 32px;
    background: var(--titlebar);
    border-bottom: 1px solid var(--border);
    flex: none;
  }
  .logo { width: 16px; height: 16px; margin: 0 8px 0 10px; }

  .menubar { --wails-draggable: no-drag; display: flex; height: 100%; z-index: 2; }
  .menu { position: relative; display: flex; }
  .top { padding: 0 8px; height: 100%; color: var(--text); border-radius: 0; }
  .top:hover, .top.active { background: var(--hover); }

  .dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    min-width: 240px;
    padding: 4px;
    background: var(--raised);
    border: 1px solid var(--border);
    border-radius: 6px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
  }
  .item {
    display: flex;
    justify-content: space-between;
    gap: 24px;
    width: 100%;
    padding: 4px 10px;
    border-radius: var(--radius);
    text-align: left;
  }
  .item:hover:not(:disabled) { background: var(--accent); color: var(--accent-text); }
  .item:hover:not(:disabled) .shortcut { color: var(--accent-text); }
  .shortcut { color: var(--muted); font-size: var(--small); }
  .sep { height: 1px; margin: 4px 6px; background: var(--border); }

  .title {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    max-width: 50%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: var(--small);
    pointer-events: none;
  }

  .controls { --wails-draggable: no-drag; display: flex; height: 100%; margin-left: auto; }
  .controls button { width: 46px; height: 100%; display: grid; place-items: center; color: var(--text); }
  .controls button:hover { background: var(--hover); }
  .controls .close:hover { background: #e81123; color: #fff; }
</style>
