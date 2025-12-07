import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  server: {
    port: 3000,
    host: '0.0.0.0',
    // 开发环境代理 API 请求到后端
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
  build: {
    // 生产构建输出到 dist 目录
    outDir: 'dist',
    // 资源文件使用 hash 命名以便缓存
    assetsDir: 'assets',
    sourcemap: false,
  },
});
