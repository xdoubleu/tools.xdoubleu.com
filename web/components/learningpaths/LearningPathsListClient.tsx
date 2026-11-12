import { useMemo } from 'react'
import { Link } from '@/components/ui/link'
import { useLearningPaths, useFetchLearningPathsPage } from '@/hooks/useLearningPaths'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { LearningPath } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { Button } from '@/components/ui/button'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { ItemCard } from '@/components/ui/item-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function moduleAndItemCounts(path: LearningPath) {
  const moduleCount = path.modules.length
  const itemCount = path.modules.reduce((sum, m) => sum + m.items.length, 0)
  const completedCount = path.modules.reduce(
    (sum, m) => sum + m.items.filter((i) => i.completed).length,
    0
  )
  return { moduleCount, itemCount, completedCount }
}

function LearningPathCard({ path }: { path: LearningPath }) {
  const { moduleCount, itemCount, completedCount } = moduleAndItemCounts(path)
  return (
    <ItemCard
      href={`/learningpaths/${path.id}`}
      title={path.title}
      meta={path.goal || undefined}
      status={
        <span className="text-xs text-muted">
          {moduleCount} module{moduleCount === 1 ? '' : 's'}
          {itemCount > 0 && (
            <>
              {' '}
              &middot; {completedCount}/{itemCount} done
            </>
          )}
        </span>
      }
      actions={
        <Button asChild variant="secondary" size="sm">
          <Link href={`/learningpaths/${path.id}/edit`}>Edit</Link>
        </Button>
      }
    />
  )
}

export default function LearningPathsListClient() {
  const { data, error, isLoading } = useLearningPaths()

  const fetchPage = useFetchLearningPathsPage()
  const initialPage = useMemo(
    () => ({ items: data?.learningPaths ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const {
    items: learningPaths,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(initialPage, fetchPage, (a, b) => a.id === b.id)

  return (
    <PageContainer>
      <PageHeader
        title="Learning paths"
        actions={
          <>
            <PageHeaderSettingsLink href="/learningpaths/settings" />
            <Button asChild>
              <Link href="/learningpaths/new">New learning path</Link>
            </Button>
          </>
        }
      />

      {isLoading && <LoadingState placement="page" label="learning paths" />}
      {error && <ErrorState placement="page" what="learning paths" />}
      {data && learningPaths.length === 0 && (
        <EmptyState placement="page">No learning paths yet. Create your first one!</EmptyState>
      )}
      {data && learningPaths.length > 0 && (
        <>
          <CollectionGrid>
            {learningPaths.map((path) => (
              <LearningPathCard key={path.id} path={path} />
            ))}
          </CollectionGrid>
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </PageContainer>
  )
}
