import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    // The Go binary embeds this directory. go:embed cannot traverse upwards,
    // so the build has to land inside the package that embeds it.
    outDir: fileURLToPath(new URL("../internal/web/dist", import.meta.url)),
    // Not emptied on build: that directory holds a committed .gitkeep which
    // keeps go:embed satisfied on a checkout with no frontend build, and
    // wiping it would make the Go build fail in a thoroughly confusing way.
    // `make frontend` clears stale output explicitly instead.
    emptyOutDir: false,
  },
  server: {
    port: 5173,
    // In development the SPA runs on Vite's server while the API runs on the
    // Go binary. Proxying keeps the browser on a single origin, so cookies and
    // the same-origin checks behave exactly as they do in production.
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8099",
        changeOrigin: false,
      },
    },
  },
});
