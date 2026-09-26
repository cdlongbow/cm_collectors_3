import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueJsx(),
    //vueDevTools(),
  ],
  optimizeDeps: {
    // 只扫描应用入口，避免 test-results 中的临时 HTML 引用失效的开发缓存。
    entries: ['index.html'],
  },
  build: {
    assetsDir: 'assets', // 静态资源目录
    emptyOutDir: true // 清空输出目录
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:12345',
        changeOrigin: true,
      },
    }
  }
})
