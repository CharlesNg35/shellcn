import { defineConfig } from "vite";

export default defineConfig({
  base: "/shellcn/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
