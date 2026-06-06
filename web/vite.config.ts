import fs from "node:fs"
import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig, type Plugin } from "vite"

// dist/.gitkeep is the committed placeholder that keeps the Go embed pattern
// (`//go:embed all:dist`) valid without committing build artifacts. Vite
// empties dist/ on every build, so recreate the placeholder afterwards —
// otherwise it silently disappears from the working tree and the next commit
// breaks `go vet` / `go build` in CI.
function preserveGitkeep(): Plugin {
  return {
    name: "preserve-gitkeep",
    closeBundle() {
      fs.writeFileSync(path.resolve(__dirname, "dist/.gitkeep"), "")
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), preserveGitkeep()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    // During development the Go backend (spr-web --no-open) serves the API.
    proxy: {
      "/api": {
        target: "http://127.0.0.1:7780",
        changeOrigin: false,
      },
    },
  },
})
