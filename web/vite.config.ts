import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 构建产物打进 Go 二进制（internal/webui/dist 由 //go:embed all:dist 消费）。
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    // 开发期：Vite dev server 反代 /api 到本地 Go 服务
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
