import { defineConfig } from 'vite'
import elmPlugin from 'vite-plugin-elm'

export default defineConfig({
  plugins: [elmPlugin()],
  server: {
    port: 3000,
    open: true,
    cors: true
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    minify: 'esbuild',
    sourcemap: true
  }
})
