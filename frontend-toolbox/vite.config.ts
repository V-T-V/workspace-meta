import { defineConfig } from 'vite';

// 前端工具箱 —— 纯前端静态站，无后端。
// 构建产物为可双击打开的静态文件（相对路径 base）。
export default defineConfig({
  base: './',
  build: {
    outDir: 'dist',
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5230,
    open: true,
  },
});
