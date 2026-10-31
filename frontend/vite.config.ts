import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite writes to dist/app, so that the committed dist/index.html placeholder stays unchanged.
export default defineConfig({
  plugins: [vue()],
  build: { outDir: 'dist/app', emptyOutDir: true },
})
