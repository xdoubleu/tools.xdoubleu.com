'use client'

import { useMemo } from 'react'
import Link from 'next/link'
import { useLearningPaths, useFetchLearningPathsPage } from '@/hooks/useLearningPaths'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import type { LearningPath } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { cn } from '@/lib/cn'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { PageContainer } from '@/components/ui/page-container'

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
    <Link
      href={`/learningpaths/${path.id}`}
      className={cn(interactiveCardClass, 'relative block p-4')}
    >
      <CardLinkStatus />
      <h2 className="font-semibold text-lg">{path.title}</h2>
      {path.goal && <p className="text-sm text-muted mt-1">{path.goal}</p>}
      <p className="text-xs text-muted mt-2">
        {moduleCount} module{moduleCount === 1 ? '' : 's'}
        {itemCount > 0 && (
          <>
            {' '}
            &middot; {completedCount}/{itemCount} done
          </>
        )}
      </p>
    </Link>
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
    <PageContainer className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">Learning Paths</h1>
        <Button asChild>
          <Link href="/learningpaths/new">New Learning Path</Link>
        </Button>
      </div>

      {isLoading && <p className="text-muted">Loading learning paths…</p>}
      {error && <p className="text-danger">Failed to load learning paths.</p>}
      {data && learningPaths.length === 0 && (
        <p className="text-muted">No learning paths yet. Create your first one!</p>
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
