import AppGrid from '@/components/AppGrid'
import { PageContainer } from '@/components/ui/page-container'

export default function DashboardsPage() {
  return (
    <PageContainer className="p-6">
      <h1 className="mb-4 text-3xl font-bold">Dashboards</h1>
      <AppGrid
        apps={[
          {
            name: 'games',
            label: 'Games',
            href: '/dashboard/games',
            description: 'Steam backlog, progress and distribution.'
          },
          {
            name: 'reading',
            label: 'Reading',
            href: '/dashboard/reading',
            description: 'Search, library and reading progress.'
          }
        ]}
      />
    </PageContainer>
  )
}
