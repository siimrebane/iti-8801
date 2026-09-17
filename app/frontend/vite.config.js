import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The Go binary serves the built files and forwards /api to the backend.
// In development, Vite forwards /api to a locally running backend instead.
export default defineConfig({
  plugins: [vue()],
  build: { outDir: '../dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8080' } },
})
