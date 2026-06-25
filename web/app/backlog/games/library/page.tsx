import GamesLibrary from '@/components/backlog/GamesLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function BacklogGamesLibraryPage() {
  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Games', href: '/backlog/games' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <GamesLibrary />
    </PageContainer>
  )
}
