import { useRouteParams } from '@/hooks/navigation'
import { useSharedSteam } from '@/hooks/useDashboardShare'
import GamesDashboardPublicClient from '@/components/dashboard/GamesDashboardPublicClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function GamesDashboardPublicPage() {
  const { token } = useRouteParams<{ token: string }>()
  const steam = useSharedSteam(token).data

  return (
    <PageContainer
      size="wide"
      className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden"
    >
      <PageHeader
        title={steam?.displayName ? `${steam.displayName}'s games` : 'Shared games'}
        className="lg:mb-3"
      />
      <GamesDashboardPublicClient token={token} />
    </PageContainer>
  )
}
