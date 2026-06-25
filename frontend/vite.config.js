/// <reference types="vitest" />
/* eslint-env node */
import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    {
      name: 'bypass-notebooks-middleware',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url && req.url.startsWith('/notebooks/')) {
            res.statusCode = 404
            res.end('Not Found')
            return
          }
          next()
        })
      }
    }
  ],
  server: {
    watch: {
      ignored: ['**/wailsjs/**']
    }
  },
  build: {
    emptyOutDir: false
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  test: {
    globals: true,
    environment: 'jsdom'
  }
})
