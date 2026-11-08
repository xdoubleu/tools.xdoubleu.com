import { PageContainer } from '@/components/ui/page-container'
import { LoadingState } from '@/components/ui/states'

export function PageLoading() {
  return (
    <PageContainer className="flex min-h-[50dvh] items-center justify-center">
      <LoadingState />
    </PageContainer>
  )
}
