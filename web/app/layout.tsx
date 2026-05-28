import type { Metadata } from 'next'
import './globals.css'
import Footer from '@/components/Footer'
import Navbar from '@/components/Navbar'

export const dynamic = 'force-dynamic'

export const metadata: Metadata = {
  title: 'tools.xdoubleu.com',
  description: 'Personal tools suite'
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
        <script
          dangerouslySetInnerHTML={{
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN: process.env.SENTRY_DSN ?? '', RELEASE: process.env.RELEASE ?? 'dev' })}`
          }}
        />
      </head>
      <body className="flex min-h-screen flex-col bg-bg text-fg">
        <Navbar />
        <main className="flex-1 px-4 py-6 sm:px-6">
          <div className="mx-auto max-w-7xl">{children}</div>
        </main>
        <Footer />
      </body>
    </html>
  )
}
