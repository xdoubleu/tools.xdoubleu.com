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
        <script
          dangerouslySetInnerHTML={{
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN: process.env.SENTRY_DSN ?? '', RELEASE: process.env.RELEASE ?? 'dev' })}`
          }}
        />
      </head>
      <body className="flex flex-col min-h-screen bg-bg text-fg">
        <Navbar />
        <main className="flex-1">{children}</main>
        <Footer />
      </body>
    </html>
  )
}
