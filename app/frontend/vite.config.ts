import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    // The logo lives in docs/assets (single source); let the dev server read it.
    fs: { allow: ["..", "../../docs/assets"] },
  },
  plugins: [svelte(), wails("./bindings")],
});
