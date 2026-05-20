import type { Metadata } from 'next'
import Link from 'next/dist/client/link'
import Footer from '@/components/Footer'

export const metadata: Metadata = {
  title: 'Todos',
  description: 'Task management'
}

export default function TodosLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col min-h-screen bg-surface">
      <header className="border-b border-border bg-card px-6 py-3">
        <nav className="flex items-center gap-4">
          <Link href="/todos" className="text-sm font-semibold text-fg hover:text-blue-600">
            Todos
          </Link>
          <Link href="/todos/settings" className="text-sm text-muted hover:text-blue-600">
            Settings
          </Link>
        </nav>
      </header>
      <main className="flex-1 mx-auto max-w-5xl px-4 py-6 w-full">{children}</main>
      <Footer />
    </div>
  )
}
