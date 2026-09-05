import type { MetadataRoute } from 'next'

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'tools.xdoubleu.com',
    short_name: 'tools',
    description: 'Personal tools suite',
    start_url: '/',
    display: 'standalone',
    background_color: '#ffffff',
    theme_color: '#7c3aed',
    icons: [
      { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml' },
      { src: '/apple-icon.png', sizes: '180x180', type: 'image/png' },
      { src: '/icon-192', sizes: '192x192', type: 'image/png' },
      { src: '/icon-512', sizes: '512x512', type: 'image/png' },
      { src: '/icon-512', sizes: '512x512', type: 'image/png', purpose: 'maskable' }
    ],
    shortcuts: [
      {
        name: 'Feeds',
        short_name: 'Feeds',
        url: '/feeds',
        icons: [{ src: '/icon-192', sizes: '192x192', type: 'image/png' }]
      },
      {
        name: 'Trains',
        short_name: 'Trains',
        url: '/trains',
        icons: [{ src: '/icon-192', sizes: '192x192', type: 'image/png' }]
      }
    ]
  }
}
