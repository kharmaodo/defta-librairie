import {defineConfig} from 'vite';

export default defineConfig({
  base: '/static/cover-imports/',
  build: {
    outDir: '../../static/cover-imports',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/app.js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]'
      }
    }
  }
});
