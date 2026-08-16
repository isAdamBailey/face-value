import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  css: ['~/assets/css/main.css'],
  ssr: false,

  app: {
    head: {
      title: 'Face Value',
      meta: [
        { name: 'description', content: 'Photo in, price out. Identifies items with a vision model and prices them against live eBay listings.' }
      ]
    }
  },

  vite: {
    plugins: [tailwindcss()]
  },

  modules: ['@nuxt/eslint', '@pinia/nuxt'],

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE
        || (process.env.NODE_ENV === 'production' ? '' : 'http://localhost:8080')
    }
  }
})
