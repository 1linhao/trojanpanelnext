import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue2'
import JavaScriptObfuscator from 'javascript-obfuscator'

const resolve = (directory) => fileURLToPath(new URL(directory, import.meta.url))

// Keep the previous production-only obfuscation, excluding third-party code.
function obfuscateApplication() {
  return {
    name: 'tp-obfuscate-application',
    apply: 'build',
    enforce: 'post',
    renderChunk(code, chunk) {
      if (chunk.name.startsWith('vendor-')) return null
      return {
        code: JavaScriptObfuscator.obfuscate(code, {
          rotateStringArray: true,
          seed: 1
        }).getObfuscatedCode(),
        map: null
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    base: '/',
    plugins: [vue(), obfuscateApplication()],
    resolve: {
      alias: {
        '@': resolve('./src')
      },
      extensions: ['.mjs', '.js', '.ts', '.jsx', '.tsx', '.json', '.vue'],
      dedupe: ['vue']
    },
    server: {
      host: '127.0.0.1',
      port: Number(env.port || 8888),
      strictPort: true,
      proxy: {
        // Keep /api intact: the previous proxy stripped then re-added it.
        '/api': {
          target: env.MOCK_API_TARGET || 'http://127.0.0.1:8081',
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: 'dist',
      assetsDir: 'static',
      sourcemap: false,
      cssCodeSplit: false,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('/node_modules/')) return
            if (/\/node_modules\/(?:vue|vue-i18n|vue-router|vuex)\//.test(id)) {
              return 'vendor-framework'
            }
            return 'vendor-dependencies'
          }
        }
      }
    }
  }
})
