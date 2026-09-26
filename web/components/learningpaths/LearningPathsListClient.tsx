'use client'

import { useMemo } from 'react'
import Link from 'next/link'
import { useLearningPaths, useFetchLearningPathsPage } from '@/hooks/useLearningPaths'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { LearningPath } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { Button } from '@/components/ui/button'
import { LinkCard } from '@/components/ui/link-card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
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
    <LinkCard href={`/learningpaths/${path.id}`} linkClassName="p-4">
      <h2 className="break-words text-lg font-semibold">{path.title}</h2>
      {path.goal && <p className="mt-1 break-words text-sm text-muted">{path.goal}</p>}
      <p className="text-xs text-muted mt-2">
        {moduleCount} module{moduleCount === 1 ? '' : 's'}
        {itemCount > 0 && (
          <>
            {' '}
            &middot; {completedCount}/{itemCount} done
          </>
        )}
      </p>
    </LinkCard>
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
        title="Learning Paths"
        actions={
          <Button asChild>
            <Link href="/learningpaths/new">New Learning Path</Link>
          </Button>
        }
      />

      {isLoading && <LoadingState label="learning paths" />}
      {error && <ErrorState what="learning paths" />}
      {data && learningPaths.length === 0 && (
        <EmptyState>No learning paths yet. Create your first one!</EmptyState>
      )}
      {data && learningPaths.length > 0 && (
        <>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {learningPaths.map((path) => (
              <LearningPathCard key={path.id} path={path} />
            ))}
          </div>
          {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
        </>
      )}
    </PageContainer>
  )
}
