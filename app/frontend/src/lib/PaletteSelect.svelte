<script lang="ts">
  import type { PaletteOption, PaletteChoice } from "./sprites";

  // value: see PaletteChoice. Options come from paletteOptions().
  let { options, value = $bindable("") }: { options: PaletteOption[]; value?: PaletteChoice } = $props();
</script>

{#if options.length > 1}
  <select class="field" bind:value aria-label="Palette" title="Palette used for sprites drawn with the main palette">
    {#each options as o (o.value)}
      <option value={o.value}>{o.label}</option>
    {/each}
  </select>
{/if}
