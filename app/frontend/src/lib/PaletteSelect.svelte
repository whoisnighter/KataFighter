<script lang="ts">
  import type { PaletteInfo } from "../../bindings/github.com/whoisnighter/KataFighter/app/services/models";

  // -1 = each sprite's own palette; otherwise the palette index that
  // replaces the main palette (see ViewerService.SpriteImage).
  let { palettes, value = $bindable(-1) }: { palettes: PaletteInfo[]; value?: number } = $props();
</script>

{#if palettes.length > 1}
  <select class="field" bind:value aria-label="Palette" title="Palette used for sprites drawn with the main palette">
    <option value={-1}>Palette: default</option>
    {#each palettes as p}
      <option value={p.index}>Palette {p.group},{p.number}{p.linkedTo >= 0 ? " (linked)" : ""}</option>
    {/each}
  </select>
{/if}
