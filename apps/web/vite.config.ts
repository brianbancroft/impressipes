import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: '../../dist/apps/web', emptyOutDir: true },
  server: {
    port: 4200,
    proxy: { '/api': 'http://localhost:8080' },
  },
});
