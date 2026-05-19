import type { Metadata } from 'next'
import Link from 'next/dist/client/link'

export const metadata: Metadata = {
  title: 'Todos',
  description: 'Task management'
}

export default function TodosLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-gray-50">
      <header className="border-b border-gray-200 bg-white px-6 py-3">
        <nav className="flex items-center gap-4">
          <Link href="/todos" className="text-sm font-semibold text-gray-900 hover:text-blue-600">
            Todos
          </Link>
          <Link href="/todos/settings" className="text-sm text-gray-500 hover:text-blue-600">
            Settings
          </Link>
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">{children}</main>
    </div>
  )
}
