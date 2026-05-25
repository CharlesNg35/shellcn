import { fileURLToPath } from "node:url";
import { mergeConfig, defineConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
  viteConfig({ command: "serve", mode: "test" }),
  defineConfig({
    test: {
      environment: "jsdom",
      include: ["src/**/*.{test,spec}.ts"],
      exclude: ["e2e/**", "node_modules/**"],
      setupFiles: ["src/test/setup.ts"],
      root: fileURLToPath(new URL("./", import.meta.url)),
    },
  }),
);
