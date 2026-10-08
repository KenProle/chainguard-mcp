import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vitest/config'

// Vite empties dist/ on every build. The Go binary embeds dist/ and needs at
// least one file there, so put the placeholder back after building.
const keepDistPlaceholder: Plugin = {
  name: 'keep-dist-placeholder',
  apply: 'build',
  closeBundle() {
    writeFileSync(resolve(import.meta.dirname, 'dist/.gitkeep'), '')
  },
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepDistPlaceholder],
  server: {
    // During development, run the Go server with -http 127.0.0.1:8080.
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})
