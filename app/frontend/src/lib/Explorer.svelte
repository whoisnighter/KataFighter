<script lang="ts">
  import Icon from "./Icon.svelte";
  import FileIcon, { fileIconFor, type FileIconName } from "./FileIcon.svelte";
  import type { Character, FileInfo } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  let { character, onclose, onopen }: { character: Character; onclose: () => void; onopen: (role: string) => void } = $props();

  // Files with a viewer open it on click; the rest show the overview.
  const viewable = (f: FileInfo) => !!f.path && (f.role === "sprite" || f.role === "anim");

  type Group = { label: string; icon: FileIconName; files: FileInfo[] };

  function group(role: string): string {
    if (role === "cns" || role === "cmd" || role === "stcommon" || role.startsWith("st[")) return "States & commands";
    if (role === "sprite" || role === "anim" || role.startsWith("pal")) return "Graphics";
    if (role === "sound") return "Sound";
    return "Other";
  }
  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }

  const groups = $derived.by(() => {
    const order = ["States & commands", "Graphics", "Sound", "Other"];
    const groupIcon: Record<string, FileIconName> = {
      "States & commands": "folder-states",
      Graphics: "folder-graphics",
      Sound: "folder-sound",
      Other: "folder",
    };
    const map = new Map<string, FileInfo[]>();
    const files = character.files ?? [];
    const cns = files.find((f) => f.role === "cns")?.path;
    for (const f of files) {
      // st = the cns file is common (KFM); show the file once.
      if (f.role.startsWith("st[") && f.path && f.path === cns) continue;
      const g = group(f.role);
      map.set(g, [...(map.get(g) ?? []), f]);
    }
    return order.filter((g) => map.has(g)).map((g): Group => ({ label: g, icon: groupIcon[g], files: map.get(g)! }));
  });

  let collapsed = $state<Record<string, boolean>>({});
</script>

<div class="explorer">
  <div class="heading">
    <span>Explorer</span>
    <button class="action" title="Close character (Ctrl+W)" onclick={onclose}><Icon name="close" size={14} /></button>
  </div>

  <div class="root">
    <FileIcon name="folder" />
    <span class="root-name">{character.displayName || character.name}</span>
  </div>
  <button class="row file" title={character.defPath} onclick={() => onopen("def")}>
    <FileIcon name="def" />
    <span class="name">{baseName(character.defPath)}</span>
  </button>

  {#each groups as g}
    <button class="row group" onclick={() => (collapsed[g.label] = !collapsed[g.label])}>
      <span class="chev" class:open={!collapsed[g.label]}><Icon name="chevron" size={12} /></span>
      <FileIcon name={g.icon} />
      <span>{g.label}</span>
    </button>
    {#if !collapsed[g.label]}
      {#each g.files as f}
        <button
          class="row file indent"
          class:missing={!f.path}
          title={`${f.path || "not found"}\n${f.summary}${viewable(f) ? "\nClick to open the viewer" : ""}`}
          onclick={() => onopen(viewable(f) ? f.role : "def")}
        >
          <FileIcon name={fileIconFor(f.role, f.name)} />
          <span class="name">{f.name}</span>
          <span class="desc">{f.role}</span>
        </button>
      {/each}
    {/if}
  {/each}
</div>

<style>
  .explorer { height: 100%; overflow: auto; background: var(--sidebar); padding-bottom: 8px; }
  .heading {
    display: flex; align-items: center; justify-content: space-between;
    height: 35px; padding: 0 8px 0 20px;
    font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted);
  }
  .action { display: grid; place-items: center; width: 22px; height: 22px; border-radius: var(--radius); color: var(--muted); }
  .action:hover { background: var(--hover); color: var(--text); }

  .root {
    display: flex; align-items: center; gap: 6px; height: 22px; padding: 0 12px;
    font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em;
  }
  .root-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .row { display: flex; align-items: center; gap: 6px; width: 100%; height: 22px; padding: 0 12px 0 20px; text-align: left; }
  .row:hover { background: var(--hover); }
  .group { color: var(--muted); font-size: var(--small); padding-left: 14px; }
  .chev { display: grid; place-items: center; transition: transform 0.1s; }
  .chev.open { transform: rotate(90deg); }
  .indent { padding-left: 34px; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .desc { margin-left: auto; color: var(--faint); font-size: 11px; flex: none; }
  .missing .name { color: var(--error); text-decoration: line-through; }
</style>
