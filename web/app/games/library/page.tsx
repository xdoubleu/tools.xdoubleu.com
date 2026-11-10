import GamesLibrary from '@/components/games/GamesLibrary'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function BacklogGamesLibraryPage() {
  return (
    <PageContainer>
      <PageHeader
        title="Library"
        breadcrumb={[{ label: 'Games', href: '/dashboard/games' }, { label: 'Library' }]}
      />

      <GamesLibrary />
    </PageContainer>
  )
}
