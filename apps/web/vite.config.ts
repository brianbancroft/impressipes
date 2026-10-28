import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../../dist/apps/web', emptyOutDir: true },
  server: {
    port: 4200,
    proxy: { '/api': { target: 'http://localhost:8080', rewrite: (p) => p.replace(/^\/api/, '') } },
  },
});
