import { defineConfig } from "vitest/config";
import { playwright } from "@vitest/browser-playwright";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

const chromiumPath = process.env.CHROME;

export default defineConfig({
  base: "/static/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: path.resolve(__dirname, "../cairn/src/cairn/server/static"),
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      "/projects": "http://127.0.0.1:8000",
      "/models": "http://127.0.0.1:8000",
      "/settings": "http://127.0.0.1:8000",
    },
  },
  test: {
    include: ["src/**/*.test.tsx"],
    browser: {
      enabled: true,
      headless: true,
      instances: [{ browser: "chromium" }],
      provider: playwright(chromiumPath ? { launchOptions: { executablePath: chromiumPath } } : {}),
    },
  },
});
