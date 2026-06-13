import GamesLibrary from '@/components/backlog/GamesLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogGamesLibraryPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Games', href: '/backlog/games' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <GamesLibrary />
    </main>
  )
}
