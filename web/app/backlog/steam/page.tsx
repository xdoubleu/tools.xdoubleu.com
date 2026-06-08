import GamesSection from '@/components/backlog/GamesSection'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogGamesPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Games' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Games</h1>

      <GamesSection />
    </main>
  )
}
