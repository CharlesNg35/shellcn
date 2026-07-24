import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  base: "/shellcn/",
  plugins: [vue(), tailwindcss()],
  // Distinct ports so the marketing site never collides with web/ (Vite default 5173).
  server: { port: 5175 },
  preview: { port: 4175 },
});
