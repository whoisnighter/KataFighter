<script lang="ts">
  import { tick } from "svelte";
  import { ViewerService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { SpriteInfo, SpriteSheet } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";
  import { spriteURL, visible } from "./sprites";
  import PaletteSelect from "./PaletteSelect.svelte";

  let { path, palette = $bindable(-1) }: { path: string; palette?: number } = $props();

  let sheet = $state<SpriteSheet | null>(null);
  let error = $state("");
  let filter = $state("");
  let selected = $state<SpriteInfo | null>(null);
  let zoom = $state(2);
  let showAxis = $state(true);
  let thumbs = $state<Record<string, string | null>>({});
  let preview = $state<string | null>(null);
  let grid = $state<HTMLElement>();

  const ZOOMS = [1, 2, 3, 4, 6, 8];

  $effect(() => {
    load(path);
  });

  async function load(p: string) {
    sheet = null;
    error = "";
    selected = null;
    try {
      const sh = await ViewerService.Sprites(p);
      if (p !== path) return;
      sheet = sh;
      await tick();
      selected = shown[0] ?? null;
    } catch (e) {
      error = String(e);
    }
  }

  // "200" shows group 200, "200,5" one sprite, "200-210" a range of groups.
  const shown = $derived.by(() => {
    const all = [...(sheet?.sprites ?? [])].sort((a, b) => a.group - b.group || a.number - b.number);
    const f = filter.trim();
    let m: RegExpMatchArray | null;
    if ((m = f.match(/^(\d+)\s*,\s*(\d+)$/))) return all.filter((s) => s.group === +m![1] && s.number === +m![2]);
    if ((m = f.match(/^(\d+)\s*-\s*(\d+)$/))) return all.filter((s) => s.group >= +m![1] && s.group <= +m![2]);
    if ((m = f.match(/^(\d+)$/))) return all.filter((s) => s.group === +m![1]);
    return all;
  });

  const groups = $derived.by(() => {
    const out: { group: number; sprites: SpriteInfo[] }[] = [];
    for (const s of shown) {
      if (out.at(-1)?.group !== s.group) out.push({ group: s.group, sprites: [] });
      out.at(-1)!.sprites.push(s);
    }
    return out;
  });

  const tkey = (s: SpriteInfo, pal: number) => `${s.group},${s.number},${pal}`;

  function loadThumb(s: SpriteInfo) {
    const k = tkey(s, palette);
    if (k in thumbs) return;
    thumbs[k] = null;
    spriteURL(path, s.group, s.number, palette).then((u) => (thumbs[k] = u));
  }

  $effect(() => {
    const s = selected;
    const pal = palette;
    preview = null;
    if (s) spriteURL(path, s.group, s.number, pal).then((u) => selected === s && palette === pal && (preview = u));
  });

  async function select(s: SpriteInfo | undefined) {
    if (!s) return;
    selected = s;
    await tick();
    grid?.querySelector(".tile.sel")?.scrollIntoView({ block: "nearest" });
  }

  function keydown(e: KeyboardEvent) {
    if (!selected || !grid) return;
    const i = shown.indexOf(selected);
    const cols = Math.max(1, Math.floor(grid.clientWidth / 84));
    const step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -cols, ArrowDown: cols, Home: -i, End: shown.length } as Record<string, number>;
    if (!(e.key in step)) return;
    e.preventDefault();
    select(shown[Math.min(shown.length - 1, Math.max(0, i + step[e.key]))]);
  }

  function spriteName(index: number): string {
    const s = sheet?.sprites[index];
    return s ? `${s.group},${s.number}` : `#${index}`;
  }

  // The preview box covers the sprite and its axis, which may lie outside it.
  const box = $derived.by(() => {
    if (!selected) return null;
    const { width: w, height: h, axisX: ax, axisY: ay } = selected;
    const left = Math.min(0, ax), top = Math.min(0, ay);
    return { left, top, w: Math.max(w, ax + 1) - left, h: Math.max(h, ay + 1) - top };
  });
</script>

<div class="browser">
  <div class="toolbar">
    <input class="field filter" placeholder="Filter: 200, 200,5 or 200-210" bind:value={filter} spellcheck="false" />
    {#if sheet}
      <PaletteSelect palettes={sheet.palettes} bind:value={palette} />
      <span class="muted count">{shown.length} / {sheet.sprites.length} sprites · SFF {sheet.version}</span>
    {/if}
  </div>

  {#if error}
    <p class="message error">{error}</p>
  {:else if !sheet}
    <p class="message muted">Loading sprites…</p>
  {:else}
    <div class="split">
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div class="grid" bind:this={grid} tabindex="0" onkeydown={keydown} role="listbox" aria-label="Sprites">
        {#key palette}
          {#each groups as g (g.group)}
            <div class="ghead">Group {g.group} <span class="faint">· {g.sprites.length}</span></div>
            <div class="tiles">
              {#each g.sprites as s (s.index)}
                {@const u = thumbs[tkey(s, palette)]}
                <button
                  class="tile"
                  class:sel={selected === s}
                  role="option"
                  aria-selected={selected === s}
                  title={`${s.group},${s.number} · ${s.width}×${s.height}`}
                  onclick={() => select(s)}
                  use:visible={() => loadThumb(s)}
                >
                  <span class="thumb">{#if u}<img src={u} alt="" />{/if}</span>
                  <span class="label">{s.group},{s.number}</span>
                </button>
              {/each}
            </div>
          {:else}
            <p class="message muted">No sprite matches “{filter}”.</p>
          {/each}
        {/key}
      </div>

      <div class="detail">
        {#if selected && box}
          <div class="dtool">
            <span class="title">{selected.group},{selected.number}</span>
            <label class="check"><input type="checkbox" bind:checked={showAxis} /> Axis</label>
            <select class="field" bind:value={zoom} aria-label="Zoom">
              {#each ZOOMS as z}<option value={z}>{z * 100}%</option>{/each}
            </select>
          </div>
          <div class="stage">
            <div class="box" style:width="{box.w * zoom}px" style:height="{box.h * zoom}px">
              <div
                class="sprite"
                style:left="{-box.left * zoom}px"
                style:top="{-box.top * zoom}px"
                style:width="{selected.width * zoom}px"
                style:height="{selected.height * zoom}px"
              >
                {#if preview}<img src={preview} alt="Sprite {selected.group},{selected.number}" />{/if}
              </div>
              {#if showAxis}
                <div class="axis" style:left="{(selected.axisX - box.left + 0.5) * zoom}px" style:top="{(selected.axisY - box.top + 0.5) * zoom}px"></div>
              {/if}
            </div>
          </div>
          <dl class="info">
            <dt>Size</dt><dd>{selected.width} × {selected.height}</dd>
            <dt>Axis</dt><dd>{selected.axisX}, {selected.axisY}</dd>
            <dt>Format</dt><dd>{selected.format}, {selected.depth}-bit</dd>
            {#if selected.palette >= 0}
              {@const p = sheet.palettes[selected.palette]}
              <dt>Palette</dt><dd>{p ? `${p.group},${p.number}` : selected.palette}{selected.palette === 0 && palette >= 0 ? ` → shown with ${sheet.palettes[palette].group},${sheet.palettes[palette].number}` : ""}</dd>
            {/if}
            {#if selected.linkedTo >= 0}<dt>Linked to</dt><dd>{spriteName(selected.linkedTo)}</dd>{/if}
            <dt>Index</dt><dd>{selected.index}</dd>
          </dl>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .browser { display: flex; flex-direction: column; height: 100%; }
  .toolbar { flex: none; display: flex; align-items: center; gap: 8px; padding: 6px 12px; border-bottom: 1px solid var(--border); }
  .filter { width: 220px; }
  .count { margin-left: auto; font-size: var(--small); }
  .message { padding: 16px; margin: 0; }
  .message.error { color: var(--error); }

  .split { flex: 1; min-height: 0; display: grid; grid-template-columns: 1fr minmax(280px, 40%); }
  .grid { overflow: auto; padding: 4px 12px 16px; outline: none; }
  .ghead { position: sticky; top: -4px; z-index: 1; padding: 8px 0 4px; background: var(--editor); font-size: var(--small); color: var(--muted); font-weight: 600; }
  .tiles { display: flex; flex-wrap: wrap; gap: 4px; }
  .tile { width: 80px; display: flex; flex-direction: column; align-items: stretch; padding: 3px; border-radius: var(--radius); border: 1px solid transparent; }
  .tile:hover { background: var(--hover); }
  .tile.sel { background: var(--selection); border-color: var(--accent); }
  .grid:not(:focus) .tile.sel { border-color: transparent; }
  .thumb { height: 64px; display: grid; place-items: center; overflow: hidden; border-radius: 2px; background: var(--sidebar); }
  .thumb img { max-width: 100%; max-height: 100%; image-rendering: pixelated; }
  .label { font-size: 11px; color: var(--muted); font-family: var(--mono); text-align: center; padding-top: 2px; }

  .detail { display: flex; flex-direction: column; min-height: 0; border-left: 1px solid var(--border); background: var(--sidebar); }
  .dtool { flex: none; display: flex; align-items: center; gap: 12px; padding: 6px 12px; border-bottom: 1px solid var(--border); }
  .title { font-family: var(--mono); color: var(--text-strong); margin-right: auto; }
  .check { display: flex; align-items: center; gap: 4px; color: var(--muted); }
  .stage { flex: 1; min-height: 0; overflow: auto; display: grid; place-items: center; padding: 24px; }
  .box { position: relative; flex: none; }
  .sprite {
    position: absolute;
    background-color: #2a2d3f;
    background-image: conic-gradient(#33374d 25%, transparent 0 50%, #33374d 0 75%, transparent 0);
    background-size: 16px 16px;
  }
  .sprite img { display: block; width: 100%; height: 100%; image-rendering: pixelated; }
  .axis { position: absolute; width: 0; height: 0; }
  .axis::before, .axis::after { content: ""; position: absolute; background: var(--strike); }
  .axis::before { left: -10px; top: -0.5px; width: 20px; height: 1px; }
  .axis::after { left: -0.5px; top: -10px; width: 1px; height: 20px; }

  .info { flex: none; display: grid; grid-template-columns: auto 1fr; gap: 4px 16px; margin: 0; padding: 12px; border-top: 1px solid var(--border); font-size: var(--small); }
  .info dt { color: var(--muted); }
  .info dd { margin: 0; font-family: var(--mono); }
</style>
