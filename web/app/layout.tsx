import { Suspense } from 'react'
import type { Metadata, Viewport } from 'next'
import { headers } from 'next/headers'
import './globals.css'
import AppShell from '@/components/AppShell'
import Splash from '@/components/Splash'
import { WebVitals } from '@/app/_components/web-vitals'
import { themeInitScript } from '@/lib/theme'

export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'tools.xdoubleu.com',
  description: 'Personal tools suite',
  appleWebApp: {
    capable: true,
    title: 'tools.xdoubleu.com',
    statusBarStyle: 'black-translucent'
  }
}

// Zoom stays disabled for an app-like feel (docs/convention-ui-standards.md).
export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
  viewportFit: 'cover',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#f2f2f7' },
    { media: '(prefers-color-scheme: dark)', color: '#000000' }
  ]
}

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  // Set by middleware.ts; the CSP allows only inline scripts carrying it.
  const nonce = (await headers()).get('x-nonce') ?? undefined

  return (
    // themeInitScript sets data-theme before hydration.
    <html lang="en" suppressHydrationWarning>
      <head>
        <meta name="msapplication-TileColor" content="#7c3aed" />
        <meta name="msapplication-TileImage" content="/apple-icon.png" />
        <link rel="mask-icon" href="/icon.svg" color="#7c3aed" />
        <script
          nonce={nonce}
          dangerouslySetInnerHTML={{
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN_WEB: process.env.SENTRY_DSN_WEB ?? '', RELEASE: process.env.RELEASE ?? 'dev', KOBO_GATEWAY_RELEASE: process.env.KOBO_GATEWAY_RELEASE ?? 'dev', POSTHOG_KEY: process.env.POSTHOG_KEY ?? '', POSTHOG_HOST: process.env.POSTHOG_HOST ?? '' })}`
          }}
        />
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: themeInitScript }} />
        <script
          nonce={nonce}
          dangerouslySetInnerHTML={{
            __html: `document.addEventListener('gesturestart',function(e){e.preventDefault()});document.addEventListener('gesturechange',function(e){e.preventDefault()});`
          }}
        />
      </head>
      <body className="flex min-h-dvh flex-col bg-bg text-fg">
        <WebVitals />
        <Suspense fallback={<Splash />}>
          <AppShell>{children}</AppShell>
        </Suspense>
      </body>
    </html>
  )
}
