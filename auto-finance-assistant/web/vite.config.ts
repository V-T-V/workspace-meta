import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// 构建产物输出到 internal/web/dist，供 Go //go:embed 嵌入
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../internal/web/dist'),
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      // 开发模式：前端 5173 反代后端 8080
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
