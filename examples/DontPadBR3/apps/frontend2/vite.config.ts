import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const backendURL = process.env.DONTPAD_BACKEND_URL || "http://127.0.0.1:8080";

export default defineConfig({
  base: "/",
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
      "@y-sweet/react": path.resolve(__dirname, "src/lib/collab/react.tsx"),
      "next/dynamic": path.resolve(__dirname, "src/shims/next-dynamic.tsx"),
      "next/link": path.resolve(__dirname, "src/shims/next-link.tsx"),
      "next/navigation": path.resolve(__dirname, "src/shims/next-navigation.ts"),
    },
  },
  server: {
    port: 5174,
    strictPort: false,
    proxy: {
      "/api": {
        target: backendURL,
        changeOrigin: true,
      },
      "/ws": {
        target: backendURL,
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    cssCodeSplit: true,
    sourcemap: false,
    chunkSizeWarningLimit: 900,
    modulePreload: {
      resolveDependencies(_filename, deps) {
        return deps.filter((dep) => {
          return !(
            dep.includes("collab-") ||
            dep.includes("editor-drawing-") ||
            dep.includes("editor-markdown-") ||
            dep.includes("editor-richtext-")
          );
        });
      },
    },
    rolldownOptions: {
      output: {
        codeSplitting: {
          includeDependenciesRecursively: true,
          groups: [
            {
              name: "react-vendor",
              test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/,
              priority: 100,
            },
            {
              name: "router",
              test: /node_modules[\\/]react-router/,
              priority: 90,
            },
            {
              name: "collab-yjs",
              test: /node_modules[\\/]yjs[\\/]/,
              priority: 80,
            },
            {
              name: "collab-protocols",
              test: /node_modules[\\/]y-protocols[\\/]/,
              priority: 79,
            },
            {
              name: "collab-lib0",
              test: /node_modules[\\/]lib0[\\/]/,
              priority: 78,
            },
            {
              name: "editor-richtext",
              test: /node_modules[\\/](quill|quill-cursors|parchment|y-quill)[\\/]/,
              priority: 70,
            },
            {
              name: "editor-markdown",
              test: /node_modules[\\/](@uiw|react-markdown|remark|rehype)[\\/]/,
              priority: 60,
            },
            {
              name: "editor-drawing",
              test: /node_modules[\\/](@excalidraw|@braintree|browser-fs-access|crc-32|es6-promise-pool|fractional-indexing|hachure-fill|jotai|jotai-scope|lodash\.throttle|nanoid|open-color|pako|perfect-freehand|png-|points-on-|roughjs|sliced)[\\/]/,
              priority: 50,
            },
          ],
        },
        entryFileNames: "assets/js/[name]-[hash].js",
        chunkFileNames: "assets/js/[name]-[hash].js",
        assetFileNames: "assets/[name]-[hash][extname]",
      },
    },
  },
});
