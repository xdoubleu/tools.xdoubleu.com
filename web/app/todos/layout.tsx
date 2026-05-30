import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Todos',
  description: 'Task management',
  appleWebApp: {
    capable: true,
    title: 'Todos',
    statusBarStyle: 'black-translucent'
  }
}

export default function TodosLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col flex-1">
      <main className="flex-1 mx-auto max-w-7xl px-4 py-6 w-full">{children}</main>
    </div>
  )
}
