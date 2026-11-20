<script lang="ts" module>
  // 16×16 line icons drawn for KataFighter (stroke = currentColor).
  export const icons = {
    files: "M4 1.5h5l3 3V14.5H4zM9 1.5v3h3M2 4v11.5h8",
    file: "M4 1.5h5l3 3v10H4zM9 1.5v3h3",
    def: "M4 1.5h5l3 3v10H4zM9 1.5v3h3M6 8h4M6 10.5h4",
    code: "M4 1.5h5l3 3v10H4zM9 1.5v3h3M7 8l-1.5 1.75L7 11.5M9 8l1.5 1.75L9 11.5",
    image: "M2 3h12v10H2zM2 11l3.5-3.5 3 3 2-2 3.5 3.5M10.5 6.2a.7.7 0 1 0 .01 0",
    film: "M2 3h12v10H2zM5 3v10M11 3v10M2 6h3M2 10h3M11 6h3M11 10h3",
    sound: "M2.5 6h3l3.5-3v10l-3.5-3h-3zM11 5.5a3.5 3.5 0 0 1 0 5M12.8 3.8a6 6 0 0 1 0 8.4",
    palette: "M8 2a6 6 0 1 0 0 12c1 0 1.2-.8.9-1.5-.4-.9.1-1.8 1.1-1.8H12a2 2 0 0 0 2-2A6 6 0 0 0 8 2zM5 7.5a.6.6 0 1 0 .01 0M8 5a.6.6 0 1 0 .01 0M11 7.5a.6.6 0 1 0 .01 0",
    folder: "M1.5 3.5h4.5l1.5 1.5h7v8.5h-13z",
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
