import HomeClient from '@/components/HomeClient'

export default function HomePage() {
  return (
    <main className="min-h-screen bg-gray-50">
      <header className="border-b border-gray-200 bg-white px-6 py-4">
        <h1 className="text-xl font-semibold text-gray-900">tools.xdoubleu.com</h1>
      </header>
      <div className="mx-auto max-w-4xl px-4 py-10">
        <HomeClient />
      </div>
    </main>
  )
}
