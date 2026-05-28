import type { Metadata } from 'next'
import Link from 'next/link'

export const metadata: Metadata = {
  title: 'Todos',
  description: 'Task management',
  appleWebApp: {
    capable: true,
    title: 'Todos',
    statusBarStyle: 'black-translucent',
  },
}

export default function TodosLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col flex-1">
      <header className="border-b border-border bg-card px-6 py-3">
        <nav className="flex items-center gap-4">
          <Link href="/todos" className="text-sm font-semibold text-fg hover:text-accent">
            Todos
          </Link>
          <Link href="/todos/settings" className="text-sm text-muted hover:text-accent">
            Settings
          </Link>
        </nav>
      </header>
      <main className="flex-1 mx-auto max-w-7xl px-4 py-6 w-full">{children}</main>
    </div>
  )
}
