import GamesDashboard from '@/components/backlog/GamesDashboard'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogGamesPage() {
  return (
    <main className="mx-auto max-w-6xl p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <Breadcrumb
        className="mb-4 lg:mb-2"
        items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Games' }]}
      />

      <h1 className="mb-4 text-3xl font-bold lg:mb-3 lg:text-2xl">Games</h1>

      <GamesDashboard />
    </main>
  )
}
