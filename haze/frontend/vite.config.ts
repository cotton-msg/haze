import { defineConfig } from "vitest/config"
import vue from "@vitejs/plugin-vue"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    environmentOptions: {
      jsdom: { url: "http://localhost:3000" },
    },
    coverage: {
      provider: "v8",
      include: ["src/components", "src/services", "src/stores"],
    },
  },
})
