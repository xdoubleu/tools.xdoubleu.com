import { useRouteParams } from '@/hooks/navigation'
import { useSharedSteam } from '@/hooks/useDashboardShare'
import GamesDashboardPublicLibrary from '@/components/dashboard/GamesDashboardPublicLibrary'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function GamesDashboardPublicLibraryPage() {
  const { token } = useRouteParams<{ token: string }>()
  const steam = useSharedSteam(token).data

  return (
    <PageContainer>
      <PageHeader
        title="Shared games library"
        breadcrumb={[
          {
            label: steam?.displayName ? `${steam.displayName}'s games` : 'Games',
            href: `/dashboard/games/${token}`
          },
          { label: 'Library' }
        ]}
      />

      <GamesDashboardPublicLibrary token={token} />
    </PageContainer>
  )
}
