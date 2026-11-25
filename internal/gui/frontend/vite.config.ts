import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  base: './',
  plugins: [react(), tailwindcss()],
  server: {
    host: '127.0.0.1',
    port: 9245,
    strictPort: true,
  },
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
  },
})
