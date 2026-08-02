import { PageContainer } from '@/components/ui/page-container'

export function PageLoading() {
  return (
    <PageContainer className="flex min-h-[50vh] items-center justify-center p-6">
      <p className="text-muted">Loading…</p>
    </PageContainer>
  )
}
