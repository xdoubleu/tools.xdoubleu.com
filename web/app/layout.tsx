import type { Metadata } from 'next'
import './globals.css'

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
            __html: `window.__ENV__=${JSON.stringify({ API_URL: process.env.API_URL ?? '', SENTRY_DSN: process.env.SENTRY_DSN ?? '' })}`
          }}
        />
      </head>
      <body className="bg-bg text-fg">{children}</body>
    </html>
  )
}
