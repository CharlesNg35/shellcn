import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { mockApiPlugin } from "./mock/server.ts";

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const useMock = env.VITE_API === "mock";

  return {
    base: "/",
    plugins: [vue(), tailwindcss(), ...(useMock ? [mockApiPlugin()] : [])],
    // Pre-bundle the full Lucide set so dev never re-optimizes (and reloads) when a
    // lazily-loaded panel first pulls an icon.
    optimizeDeps: {
      include: ["@lucide/vue"],
    },
    build: {
      outDir: "dist",
    },
    server: useMock
      ? {}
      : {
          proxy: {
            "/api": {
              target: env.VITE_API_TARGET || "http://localhost:8081",
              changeOrigin: false,
              ws: true,
            },
          },
        },
  };
});
