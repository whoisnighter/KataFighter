// Sprite images for the sprite browser and the animation player. The Go
// side decodes sprites (ViewerService.SpriteImage); this caches the
// results so a sprite is fetched once per palette.
import { ViewerService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";
import type { PaletteInfo } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

const urls = new Map<string, Promise<string | null>>();
const images = new Map<string, Promise<HTMLImageElement | null>>();

/**
 * Which palette sprites are shown with: "" (each sprite's own), "sff:N"
 * (palette N of the SFF) or "act:PATH" (an ACT file of the character).
 */
export type PaletteChoice = string;

/** A character palette file from the DEF (pal1..palN). */
export type ActFile = { slot: number; name: string; path: string };

export type PaletteOption = { value: PaletteChoice; label: string };

/**
 * The palettes to offer, like Ikemen builds them for a character: slot N
 * (pal N) is the DEF's ACT file when there is one, otherwise SFF palette
 * 1,N; other SFF palettes follow.
 */
export function paletteOptions(palettes: PaletteInfo[], acts: ActFile[]): PaletteOption[] {
  const out: PaletteOption[] = [{ value: "", label: "Palette: default" }];
  const used = new Set<number>();
  const slots = new Set([...acts.map((a) => a.slot), ...palettes.filter((p) => p.group === 1 && p.number > 0).map((p) => p.number)]);
  for (const slot of [...slots].sort((a, b) => a - b)) {
    const a = acts.find((a) => a.slot === slot);
    const p = palettes.find((p) => p.group === 1 && p.number === slot);
    if (p) used.add(p.index);
    if (a) out.push({ value: `act:${a.path}`, label: `pal${slot} · ${a.name}` });
    else if (p) out.push({ value: `sff:${p.index}`, label: `pal${slot} · SFF 1,${slot}` });
  }
  for (const p of palettes) {
    if (!used.has(p.index)) out.push({ value: `sff:${p.index}`, label: p.group || p.number ? `SFF ${p.group},${p.number}` : `SFF #${p.index}` });
  }
  return out;
}

function args(choice: PaletteChoice): [number, string] {
  if (choice.startsWith("sff:")) return [Number(choice.slice(4)), ""];
  if (choice.startsWith("act:")) return [-1, choice.slice(4)];
  return [-1, ""];
}

function key(path: string, group: number, number: number, choice: PaletteChoice): string {
  return `${path}|${group},${number}|${choice}`;
}

/** PNG data URL of a sprite, or null when it cannot be decoded. */
export function spriteURL(path: string, group: number, number: number, choice: PaletteChoice = ""): Promise<string | null> {
  const k = key(path, group, number, choice);
  let p = urls.get(k);
  if (!p) {
    p = ViewerService.SpriteImage(path, group, number, ...args(choice)).catch(() => null);
    urls.set(k, p);
  }
  return p;
}

/** The sprite as a loaded image, for drawing on a canvas. */
export function spriteImage(path: string, group: number, number: number, pal: PaletteChoice = ""): Promise<HTMLImageElement | null> {
  const k = key(path, group, number, pal);
  let p = images.get(k);
  if (!p) {
    p = spriteURL(path, group, number, pal).then((url) =>
      !url
        ? null
        : new Promise<HTMLImageElement | null>((resolve) => {
          const img = new Image();
          img.onload = () => resolve(img);
          img.onerror = () => resolve(null);
          img.src = url;
        }),
    );
    images.set(k, p);
  }
  return p;
}

/** Svelte action: calls onvisible once, when the element scrolls into view. */
export function visible(node: Element, onvisible: () => void) {
  const io = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting)) {
        io.disconnect();
        onvisible();
      }
    },
    { rootMargin: "200px" },
  );
  io.observe(node);
  return { destroy: () => io.disconnect() };
}

/** Reads a CSS variable from the document (for canvas drawing). */
export function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}
