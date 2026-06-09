import GamesDashboard from '@/components/backlog/GamesDashboard'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogGamesPage() {
  return (
    <main className="mx-auto max-w-6xl p-6 lg:flex lg:h-dvh lg:flex-col">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Games' }]}
      />

      <h1 className="mb-4 text-3xl font-bold">Games</h1>

      <GamesDashboard />
    </main>
  )
}
