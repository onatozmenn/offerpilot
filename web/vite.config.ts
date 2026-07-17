import react from "@vitejs/plugin-react";
import { loadEnv } from "vite";
import { defineConfig } from "vitest/config";

const defaultProxyTarget = "http://127.0.0.1:8080";

function requireOrigin(name: string, value: string): string {
  let url: URL;

  try {
    url = new URL(value);
  } catch {
    throw new Error(`${name} must be an absolute HTTP or HTTPS origin.`);
  }

  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    url.username !== "" ||
    url.password !== "" ||
    url.pathname !== "/" ||
    url.search !== "" ||
    url.hash !== ""
  ) {
    throw new Error(`${name} must be an absolute HTTP or HTTPS origin without credentials or a path.`);
  }

  return url.origin;
}

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, process.cwd(), "");
  const proxyTarget = requireOrigin(
    "OFFERPILOT_API_PROXY_TARGET",
    environment.OFFERPILOT_API_PROXY_TARGET || defaultProxyTarget,
  );
  const publicApiBaseUrl = environment.VITE_API_BASE_URL?.trim() ?? "";

  if (publicApiBaseUrl !== "") {
    requireOrigin("VITE_API_BASE_URL", publicApiBaseUrl);
  }

  return {
    plugins: [react()],
    server: {
      host: "127.0.0.1",
      port: 5173,
      strictPort: true,
      proxy: {
        "/v1": proxyTarget,
        "/health": proxyTarget,
        "/metrics": proxyTarget,
      },
    },
    preview: {
      host: "127.0.0.1",
      port: 4173,
      strictPort: true,
    },
    build: {
      outDir: "dist",
      target: "es2022",
    },
    test: {
      environment: "jsdom",
      setupFiles: "./src/test/setup.ts",
      css: true,
      coverage: {
        provider: "v8",
        reporter: ["text", "html", "lcov"],
        exclude: ["src/main.tsx", "src/types/**"],
      },
    },
  };
});