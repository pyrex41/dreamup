import { defineConfig } from 'vite'
import elmPlugin from 'vite-plugin-elm'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    elmPlugin(),
    tailwindcss(),
  ],
  server: {
    port: 3000,
    open: true,
    cors: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    minify: 'esbuild',
    sourcemap: true
  }
})
