<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { ViewerService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";
  import type { AnimAction, AnimFrame, SpriteInfo, SpriteSheet } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";
  import { cssVar, spriteImage } from "./sprites";
  import Icon from "./Icon.svelte";
  import PaletteSelect from "./PaletteSelect.svelte";

  let { animPath, spritePath, palette = $bindable(-1) }: { animPath: string; spritePath: string; palette?: number } = $props();

  const TICKS_PER_SECOND = 60; // Ikemen's game speed
  const ZOOMS = [1, 2, 3, 4];
  const SPEEDS = [0.25, 0.5, 1, 2];

  let actions = $state<AnimAction[]>([]);
  let sheet = $state<SpriteSheet | null>(null);
  let error = $state("");
  let loading = $state(true);
  let filter = $state("");
  let action = $state<AnimAction | null>(null);

  // Playback: frame index, ticks spent in it, ticks since the action began.
  let frame = $state(0);
  let ticks = $state(0);
  let elapsed = $state(0);
  let playing = $state(false);
  let loop = $state(true);
  let speed = $state(1);

  let zoom = $state(2);
  let showClsn = $state(true);
  let showOnion = $state(false);
  let showAxis = $state(true);
  let pan = $state({ x: 0, y: 0 });

  let canvas = $state<HTMLCanvasElement>();
  let size = $state({ w: 0, h: 0 });
  let images = new Map<string, HTMLImageElement | null>();
  let loaded = $state(0); // bumped when an image arrives, to redraw
  let list = $state<HTMLElement>();

  $effect(() => {
    load(animPath, spritePath);
  });

  async function load(anim: string, spr: string) {
    loading = true;
    error = "";
    try {
      const [acts, sh] = await Promise.all([ViewerService.Actions(anim), ViewerService.Sprites(spr).catch(() => null)]);
      actions = acts;
      sheet = sh;
      choose(acts.find((a) => a.number === 0) ?? acts[0] ?? null);
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  }

  const sprites = $derived.by(() => {
    const m = new Map<string, SpriteInfo>();
    for (const s of sheet?.sprites ?? []) {
      const k = `${s.group},${s.number}`;
      if (!m.has(k)) m.set(k, s); // the first sprite wins, as in Ikemen
    }
    return m;
  });

  const shown = $derived.by(() => {
    const f = filter.trim();
    return f ? actions.filter((a) => String(a.number).startsWith(f)) : actions;
  });

  const frames = $derived(action?.frames ?? []);
  const current = $derived<AnimFrame | undefined>(frames[frame]);

  // Load the images of the action's frames (and reload on palette change).
  $effect(() => {
    const pal = palette;
    for (const f of frames) {
      const k = `${f.group},${f.number},${pal}`;
      if (images.has(k)) continue;
      images.set(k, null);
      spriteImage(spritePath, f.group, f.number, pal).then((img) => {
        images.set(k, img);
        loaded++;
      });
    }
  });

  async function choose(a: AnimAction | null) {
    action = a;
    frame = 0;
    ticks = 0;
    elapsed = 0;
    await tick();
    list?.querySelector(".act.sel")?.scrollIntoView({ block: "nearest" });
  }

  function chooseRelative(d: number) {
    const i = action ? shown.indexOf(action) : -1;
    const next = shown[Math.min(shown.length - 1, Math.max(0, i + d))];
    if (next) choose(next);
  }

  // One game tick, following Ikemen's frame timing: a frame shows for Time
  // ticks (-1 = forever); after the last frame the action restarts at the
  // loop start.
  function step() {
    const f = frames[frame];
    if (!f || f.time === -1) {
      playing = false;
      return;
    }
    ticks++;
    elapsed++;
    for (let guard = 0; ticks >= frames[frame].time && frames[frame].time !== -1 && guard <= frames.length; guard++) {
      ticks = 0;
      if (frame + 1 < frames.length) frame++;
      else if (loop) {
        frame = action!.loopStart;
        elapsed = frames.slice(0, frame).reduce((t, f) => t + Math.max(0, f.time), 0);
      } else {
        playing = false;
        return;
      }
    }
  }

  function goto(i: number) {
    if (!frames.length) return;
    frame = (i + frames.length) % frames.length;
    ticks = 0;
    elapsed = frames.slice(0, frame).reduce((t, f) => t + Math.max(0, f.time), 0);
  }

  let raf = 0;
  let last = 0;
  let acc = 0;
  $effect(() => {
    if (!playing) return;
    last = performance.now();
    acc = 0;
    const run = (now: number) => {
      acc += ((now - last) / 1000) * TICKS_PER_SECOND * speed;
      last = now;
      for (; acc >= 1 && playing; acc--) step();
      if (playing) raf = requestAnimationFrame(run);
    };
    raf = requestAnimationFrame(run);
    return () => cancelAnimationFrame(raf);
  });
  onDestroy(() => cancelAnimationFrame(raf));

  function togglePlay() {
    if (!playing && current?.time === -1) goto(loop ? action!.loopStart : 0);
    playing = !playing;
  }

  // Canvas size follows its container.
  $effect(() => {
    if (!canvas) return;
    const ro = new ResizeObserver(([e]) => (size = { w: e.contentRect.width, h: e.contentRect.height }));
    ro.observe(canvas.parentElement!);
    return () => ro.disconnect();
  });

  $effect(() => {
    void [loaded, frame, zoom, showClsn, showOnion, showAxis, pan, size, palette, action];
    draw();
  });

  function draw() {
    if (!canvas || !size.w) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(size.w * dpr);
    canvas.height = Math.round(size.h * dpr);
    const ctx = canvas.getContext("2d")!;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.imageSmoothingEnabled = false;
    ctx.clearRect(0, 0, size.w, size.h);

    const ox = Math.round(size.w / 2 + pan.x);
    const oy = Math.round(size.h * 0.75 + pan.y);

    // Ground line and center line.
    ctx.strokeStyle = cssVar("--border");
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(0, oy + 0.5);
    ctx.lineTo(size.w, oy + 0.5);
    ctx.moveTo(ox + 0.5, 0);
    ctx.lineTo(ox + 0.5, size.h);
    ctx.stroke();

    if (showOnion && frames.length > 1) {
      const prev = frame > 0 ? frame - 1 : frames.length - 1;
      drawFrame(ctx, frames[prev], ox, oy, 0.3);
    }
    if (current) drawFrame(ctx, current, ox, oy, 1);

    if (showClsn && current) {
      drawBoxes(ctx, current.clsn2 ?? [], ox, oy, cssVar("--accent"));
      drawBoxes(ctx, current.clsn1 ?? [], ox, oy, cssVar("--strike"));
    }
    if (showAxis) {
      ctx.strokeStyle = cssVar("--strike-2");
      ctx.beginPath();
      ctx.moveTo(ox - 8, oy + 0.5);
      ctx.lineTo(ox + 9, oy + 0.5);
      ctx.moveTo(ox + 0.5, oy - 8);
      ctx.lineTo(ox + 0.5, oy + 9);
      ctx.stroke();
    }
  }

  // The sprite axis goes to the character position plus the frame offset.
  // Flips and scale apply around the axis; blending is approximated with
  // canvas composite modes.
  function drawFrame(ctx: CanvasRenderingContext2D, f: AnimFrame, ox: number, oy: number, alpha: number) {
    const info = sprites.get(`${f.group},${f.number}`);
    const img = images.get(`${f.group},${f.number},${palette}`);
    if (!info || !img) return;
    ctx.save();
    ctx.translate(ox + f.offsetX * zoom, oy + f.offsetY * zoom);
    if (f.angle) ctx.rotate((-f.angle * Math.PI) / 180);
    ctx.scale(zoom * f.scaleX * (f.flipH ? -1 : 1), zoom * f.scaleY * (f.flipV ? -1 : 1));
    ctx.globalAlpha = alpha * (f.trans === "none" ? 1 : f.srcAlpha / 255);
    ctx.globalCompositeOperation = f.trans === "add" || f.trans === "subadd" ? "lighter" : f.trans === "sub" ? "difference" : "source-over";
    ctx.drawImage(img, -info.axisX, -info.axisY);
    ctx.restore();
  }

  function drawBoxes(ctx: CanvasRenderingContext2D, boxes: number[][], ox: number, oy: number, color: string) {
    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    for (const [l, t, r, b] of boxes) {
      const x = ox + l * zoom, y = oy + t * zoom, w = (r - l) * zoom, h = (b - t) * zoom;
      ctx.globalAlpha = 0.18;
      ctx.fillRect(x, y, w, h);
      ctx.globalAlpha = 1;
      ctx.strokeRect(Math.round(x) + 0.5, Math.round(y) + 0.5, Math.round(w) - 1, Math.round(h) - 1);
    }
  }

  // Drag to pan, wheel to zoom.
  let drag: { x: number; y: number; px: number; py: number } | null = null;
  function pointerdown(e: PointerEvent) {
    drag = { x: e.clientX, y: e.clientY, px: pan.x, py: pan.y };
    (e.currentTarget as Element).setPointerCapture(e.pointerId);
  }
  function pointermove(e: PointerEvent) {
    if (drag) pan = { x: drag.px + e.clientX - drag.x, y: drag.py + e.clientY - drag.y };
  }
  function wheel(e: WheelEvent) {
    e.preventDefault();
    const i = ZOOMS.indexOf(zoom) + (e.deltaY < 0 ? 1 : -1);
    zoom = ZOOMS[Math.min(ZOOMS.length - 1, Math.max(0, i))];
  }

  function keydown(e: KeyboardEvent) {
    if ((e.target as Element).tagName === "INPUT" || (e.target as Element).tagName === "SELECT") return;
    if (e.key === " ") togglePlay();
    else if (e.key === "ArrowLeft") (playing = false), goto(frame - 1);
    else if (e.key === "ArrowRight") (playing = false), goto(frame + 1);
    else if (e.key === "ArrowUp") chooseRelative(-1);
    else if (e.key === "ArrowDown") chooseRelative(1);
    else return;
    e.preventDefault();
  }

  const time = (t: number) => (t === -1 ? "∞" : String(t));
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div class="player" tabindex="0" role="application" aria-label="Animation player" onkeydown={keydown}>
  <div class="actions">
    <input class="field filter" placeholder="Action number" bind:value={filter} spellcheck="false" />
    <div class="list" bind:this={list}>
      {#each shown as a (a.number)}
        <button class="act" class:sel={a === action} onclick={() => choose(a)} title={`Line ${a.line}${a.copiedFrom >= 0 ? ` · copy of ${a.copiedFrom}` : ""}`}>
          <span class="num">{a.number}</span>
          <span class="faint">{a.frames.length}f · {time(a.totalTime)}</span>
        </button>
      {:else}
        <p class="muted empty">{loading ? "Loading…" : "No actions."}</p>
      {/each}
    </div>
  </div>

  <div class="main">
    <div class="toolbar">
      <button class="tool" title="Previous frame (←)" onclick={() => ((playing = false), goto(frame - 1))}><Icon name="step-back" /></button>
      <button class="tool play" title={playing ? "Pause (Space)" : "Play (Space)"} onclick={togglePlay}><Icon name={playing ? "pause" : "play"} /></button>
      <button class="tool" title="Next frame (→)" onclick={() => ((playing = false), goto(frame + 1))}><Icon name="step-forward" /></button>
      <span class="tickinfo mono">tick {elapsed}{action ? ` / ${time(action.totalTime)}` : ""}</span>
      <span class="sep"></span>
      <label class="check"><input type="checkbox" bind:checked={loop} /> Loop</label>
      <label class="check"><input type="checkbox" bind:checked={showClsn} /> Clsn</label>
      <label class="check"><input type="checkbox" bind:checked={showOnion} /> Onion skin</label>
      <label class="check"><input type="checkbox" bind:checked={showAxis} /> Axis</label>
      <span class="sep"></span>
      <select class="field" bind:value={speed} aria-label="Speed">
        {#each SPEEDS as s}<option value={s}>{s}×</option>{/each}
      </select>
      <select class="field" bind:value={zoom} aria-label="Zoom">
        {#each ZOOMS as z}<option value={z}>{z * 100}%</option>{/each}
      </select>
      {#if sheet}<PaletteSelect palettes={sheet.palettes} bind:value={palette} />{/if}
      <button class="tool" title="Center view" onclick={() => (pan = { x: 0, y: 0 })}>⌖</button>
    </div>

    {#if error}
      <p class="message error">{error}</p>
    {:else}
      <div class="stage">
        <canvas
          bind:this={canvas}
          style:width="{size.w}px"
          style:height="{size.h}px"
          onpointerdown={pointerdown}
          onpointermove={pointermove}
          onpointerup={() => (drag = null)}
          onwheel={wheel}
        ></canvas>
        {#if action && !frames.length}<p class="overlay muted">Action {action.number} has no frames.</p>{/if}
        {#if current && !sprites.has(`${current.group},${current.number}`) && current.group >= 0}
          <p class="overlay error">Sprite {current.group},{current.number} is not in the SFF.</p>
        {/if}
      </div>

      {#if action && frames.length}
        <div class="timeline">
          {#each frames as f, i}
            <button
              class="cell"
              class:sel={i === frame}
              class:loopstart={i === action.loopStart && action.loopStart > 0}
              style:flex-grow={f.time === -1 ? 12 : Math.max(1, f.time)}
              title={`Frame ${i + 1}: sprite ${f.group},${f.number}, ${time(f.time)} ticks${i === action.loopStart ? " · loop start" : ""}`}
              onclick={() => ((playing = false), goto(i))}
            >
              {i + 1}
            </button>
          {/each}
        </div>
        {#if current}
          <div class="frameinfo mono">
            <span>Frame {frame + 1}/{frames.length}</span>
            <span>Sprite {current.group},{current.number}</span>
            <span>Offset {current.offsetX},{current.offsetY}</span>
            <span>Time {time(current.time)}</span>
            {#if current.flipH || current.flipV}<span>Flip {current.flipH ? "H" : ""}{current.flipV ? "V" : ""}</span>{/if}
            {#if current.trans !== "none"}<span>Blend {current.trans} {current.srcAlpha}/{current.dstAlpha}</span>{/if}
            {#if current.scaleX !== 1 || current.scaleY !== 1}<span>Scale {current.scaleX},{current.scaleY}</span>{/if}
            {#if current.angle}<span>Angle {current.angle}</span>{/if}
            <span class="clsn2">Clsn2 {current.clsn2?.length ?? 0}</span>
            <span class="clsn1">Clsn1 {current.clsn1?.length ?? 0}</span>
            <span class="faint">line {current.line}</span>
          </div>
        {/if}
      {/if}
    {/if}
  </div>
</div>

<style>
  .player { height: 100%; display: grid; grid-template-columns: 180px 1fr; outline: none; }

  .actions { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--border); background: var(--sidebar); }
  .filter { margin: 6px 8px; }
  .list { flex: 1; overflow: auto; }
  .act { display: flex; align-items: baseline; justify-content: space-between; width: 100%; height: 22px; padding: 0 10px; font-size: var(--small); }
  .act:hover { background: var(--hover); }
  .act.sel { background: var(--selection); }
  .num { font-family: var(--mono); color: var(--text-strong); }
  .empty { padding: 8px 10px; margin: 0; }

  .main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  .toolbar { flex: none; display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px; padding: 5px 10px; border-bottom: 1px solid var(--border); }
  .tool { display: grid; place-items: center; width: 26px; height: 24px; border-radius: var(--radius); color: var(--text); }
  .tool:hover { background: var(--hover); }
  .tool.play { color: var(--accent); }
  .tickinfo { color: var(--muted); min-width: 90px; }
  .sep { width: 1px; height: 16px; background: var(--border); }
  .check { display: flex; align-items: center; gap: 4px; color: var(--muted); }

  .stage { position: relative; flex: 1; min-height: 0; overflow: hidden; background: var(--editor); }
  canvas { position: absolute; inset: 0; cursor: grab; touch-action: none; }
  canvas:active { cursor: grabbing; }
  .overlay { position: absolute; top: 8px; left: 12px; margin: 0; pointer-events: none; }
  .message { padding: 16px; margin: 0; }
  .error { color: var(--error); }

  .timeline { flex: none; display: flex; gap: 2px; padding: 6px 10px 0; border-top: 1px solid var(--border); background: var(--sidebar); }
  .cell {
    flex-basis: 0; min-width: 22px; height: 22px; border-radius: 2px;
    background: var(--raised); color: var(--muted); font-size: 11px; font-family: var(--mono);
  }
  .cell:hover { background: var(--hover); color: var(--text); }
  .cell.sel { background: var(--accent); color: var(--accent-text); }
  .cell.loopstart { box-shadow: inset 2px 0 0 var(--strike-2); }
  .frameinfo { flex: none; display: flex; flex-wrap: wrap; gap: 4px 16px; padding: 6px 10px 8px; background: var(--sidebar); color: var(--text); }
  .clsn1 { color: var(--strike); }
  .clsn2 { color: var(--accent); }
</style>
