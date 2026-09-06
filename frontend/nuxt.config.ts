import tailwindcss from '@tailwindcss/vite'

const themeColor = '#eef0f2'
const appDescription = 'Photo in, price out. Identifies items with a vision model and prices them against live eBay listings.'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  css: ['~/assets/css/main.css'],
  ssr: false,

  app: {
    head: {
      title: 'Face Value',
      viewport: 'width=device-width, initial-scale=1, viewport-fit=cover',
      meta: [
        { name: 'description', content: appDescription },
        { name: 'theme-color', content: themeColor },
        { name: 'mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-status-bar-style', content: 'default' },
        { name: 'apple-mobile-web-app-title', content: 'Face Value' }
      ],
      link: [
        { rel: 'icon', href: '/favicon.ico', sizes: '48x48' },
        { rel: 'icon', href: '/logo.svg', type: 'image/svg+xml' },
        { rel: 'apple-touch-icon', href: '/apple-touch-icon-180x180.png' },
        { rel: 'manifest', href: '/manifest.webmanifest' }
      ]
    }
  },

  vite: {
    plugins: [tailwindcss()]
  },

  modules: ['@nuxt/eslint', '@pinia/nuxt', '@vite-pwa/nuxt'],

  pwa: {
    registerType: 'autoUpdate',
    includeAssets: [
      'favicon.ico',
      'logo.svg',
      'apple-touch-icon-180x180.png',
      'fonts/*.woff2'
    ],
    manifest: {
      id: '/',
      name: 'Face Value',
      short_name: 'Face Value',
      description: appDescription,
      theme_color: themeColor,
      background_color: themeColor,
      display: 'standalone',
      start_url: '/',
      scope: '/',
      lang: 'en',
      icons: [
        {
          src: 'pwa-64x64.png',
          sizes: '64x64',
          type: 'image/png'
        },
        {
          src: 'pwa-192x192.png',
          sizes: '192x192',
          type: 'image/png'
        },
        {
          src: 'pwa-512x512.png',
          sizes: '512x512',
          type: 'image/png',
          purpose: 'any'
        },
        {
          src: 'maskable-icon-512x512.png',
          sizes: '512x512',
          type: 'image/png',
          purpose: 'maskable'
        }
      ]
    },
    workbox: {
      globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
      navigateFallback: null
    }
  },

  nitro: {
    routeRules: {
      '/sw.js': {
        headers: { 'Cache-Control': 'public, max-age=0, must-revalidate' }
      },
      '/manifest.webmanifest': {
        headers: { 'Cache-Control': 'public, max-age=0, must-revalidate' }
      }
    }
  },

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE
        || (process.env.NODE_ENV === 'production' ? '' : 'http://localhost:8080')
    }
  }
})
