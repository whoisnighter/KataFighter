<script lang="ts" module>
  // 16×16 line icons for the UI chrome, drawn for KataFighter (stroke =
  // currentColor). File and folder icons are in FileIcon.svelte.
  export const icons = {
    chevron: "M6 4l4 4-4 4",
    error: "M8 1.8a6.2 6.2 0 1 0 0 12.4A6.2 6.2 0 0 0 8 1.8zM5.8 5.8l4.4 4.4M10.2 5.8l-4.4 4.4",
    warning: "M8 2l6.5 11.5h-13zM8 6.5v3.5M8 11.8v.4",
    check: "M3 8.5l3 3 7-7",
    open: "M1.5 3.5h4.5l1.5 1.5h7v2M1.5 3.5v10h11l2-6.5H4z",
    close: "M4 4l8 8M12 4l-8 8",
    minimise: "M3 8h10",
    maximise: "M3.5 3.5h9v9h-9z",
    restore: "M3.5 5.5h7v7h-7zM5.5 5.5v-2h7v7h-2",
    sidebar: "M2 3h12v10H2zM6 3v10",
    panel: "M2 3h12v10H2zM2 10h12",
    info: "M8 1.8a6.2 6.2 0 1 0 0 12.4A6.2 6.2 0 0 0 8 1.8zM8 7v4.5M8 4.8v.4",
    play: "M5 3.5v9l7-4.5z",
    pause: "M5.5 3.5v9M10.5 3.5v9",
    "step-back": "M4 3.5v9M12.5 3.5v9L6 8z",
    "step-forward": "M12 3.5v9M3.5 3.5v9L10 8z",
  } as const;
  export type IconName = keyof typeof icons;
</script>

<script lang="ts">
  let { name, size = 16 }: { name: IconName; size?: number } = $props();
</script>

<svg class="icon" width={size} height={size} viewBox="0 0 16 16" aria-hidden="true">
  <path d={icons[name]} />
</svg>

<style>
  .icon {
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
