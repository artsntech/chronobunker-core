import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/query': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
      '/metrics': {
        target: 'http://localhost:9090',
        changeOrigin: true,
      },
    },
  },
})
