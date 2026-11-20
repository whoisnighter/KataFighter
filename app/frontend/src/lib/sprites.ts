// Sprite images for the sprite browser and the animation player. The Go
// side decodes sprites (ViewerService.SpriteImage); this caches the
// results so a sprite is fetched once per palette.
import { ViewerService } from "../../bindings/github.com/whoisnighter/KataFighter/app/services";

const urls = new Map<string, Promise<string | null>>();
const images = new Map<string, Promise<HTMLImageElement | null>>();

function key(path: string, group: number, number: number, pal: number): string {
  return `${path}|${group},${number}|${pal}`;
}

/** PNG data URL of a sprite, or null when it cannot be decoded. */
export function spriteURL(path: string, group: number, number: number, pal = -1): Promise<string | null> {
  const k = key(path, group, number, pal);
  let p = urls.get(k);
  if (!p) {
    p = ViewerService.SpriteImage(path, group, number, pal).catch(() => null);
    urls.set(k, p);
  }
  return p;
}

/** The sprite as a loaded image, for drawing on a canvas. */
export function spriteImage(path: string, group: number, number: number, pal = -1): Promise<HTMLImageElement | null> {
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
