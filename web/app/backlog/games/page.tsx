import Link from 'next/link'
import GamesDashboard from '@/components/backlog/GamesDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'

export default function BacklogGamesPage() {
  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <div className="mb-4 flex items-center justify-between gap-4 lg:mb-3">
        <h1 className="text-3xl font-bold lg:text-2xl">Games</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/backlog/games/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <GamesDashboard />
    </PageContainer>
  )
}
