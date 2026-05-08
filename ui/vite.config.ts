import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/admin/ui/" : "/",
  plugins: [react(), tailwindcss()],
  server: {
    host: "127.0.0.1",
    port: 8189,
    strictPort: true,
    proxy: {
      "/admin/v1": {
        target: process.env.VITE_SIGILBRIDGE_ADMIN_TARGET ?? process.env.VITE_SIGILBRIDGE_TARGET ?? "http://127.0.0.1:8188",
        changeOrigin: true
      },
      "/v1": {
        target: process.env.VITE_SIGILBRIDGE_TARGET ?? "http://127.0.0.1:8187",
        changeOrigin: true
      },
      "/healthz": {
        target: process.env.VITE_SIGILBRIDGE_TARGET ?? "http://127.0.0.1:8187",
        changeOrigin: true
      },
      "/readyz": {
        target: process.env.VITE_SIGILBRIDGE_TARGET ?? "http://127.0.0.1:8187",
        changeOrigin: true
      }
    }
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./tests/setup.ts"],
    exclude: ["node_modules", "dist", "tests/e2e/**"]
  }
}));
