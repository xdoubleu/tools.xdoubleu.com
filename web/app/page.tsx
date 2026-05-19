import HomeClient from '@/components/HomeClient'

export default function HomePage() {
  return (
    <main className="min-h-screen bg-surface">
      <header className="border-b border-border bg-card px-6 py-4">
        <h1 className="text-xl font-semibold text-fg">tools.xdoubleu.com</h1>
      </header>
      <div className="mx-auto max-w-4xl px-4 py-10">
        <HomeClient />
      </div>
    </main>
  )
}
