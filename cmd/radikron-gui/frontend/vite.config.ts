import path from 'path';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import wails from '@wailsio/runtime/plugins/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss(), wails('./bindings')],
  root: '.',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: './index.html',
    },
  },
  server: {
    port: 34115,
  },
  // TypeScript support
  esbuild: {
    target: 'es2020',
  },
});

