import { PageContainer } from '@/components/ui/page-container'

export function PageLoading() {
  return (
    <PageContainer className="flex min-h-[50dvh] items-center justify-center">
      <p className="text-muted">Loading…</p>
    </PageContainer>
  )
}
