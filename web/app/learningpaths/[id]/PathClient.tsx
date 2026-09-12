'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import {
  useLearningPath,
  useDeleteLearningPath,
  useRecordItemProgress
} from '@/hooks/useLearningPaths'
import type { DeleteLearningPathInput } from '@/hooks/useLearningPaths'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function PathClient({ id }: { id: string }) {
  const { data, error, isLoading, mutate } = useLearningPath(id)
  const deleteLearningPath = useDeleteLearningPath()
  const recordItemProgress = useRecordItemProgress()
  const router = useRouter()

  const [deleteConfirm, setDeleteConfirm] = useState(false)

  const learningPath = data?.learningPath

  const handleDelete = async () => {
    if (!learningPath) return
    const req: DeleteLearningPathInput = { id: learningPath.id }
    await deleteLearningPath(req)
    router.push('/learningpaths/list')
  }

  const handleToggleItem = async (itemId: string, completed: boolean) => {
    await recordItemProgress({ itemId, completed })
    await mutate()
  }

  const totalItems = learningPath?.modules.reduce((sum, m) => sum + m.items.length, 0) ?? 0
  const completedItems =
    learningPath?.modules.reduce((sum, m) => sum + m.items.filter((i) => i.completed).length, 0) ??
    0

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: learningPath?.title ?? 'Learning Path' }
        ]}
      />

      {isLoading && !learningPath && <p className="text-muted">Loading learning path…</p>}
      {error && <p className="text-danger">Failed to load learning path.</p>}
      {learningPath && (
        <>
          <div className="flex items-start justify-between mb-2">
            <h1 className="text-3xl font-bold">{learningPath.title}</h1>
            <div className="flex gap-2 ml-4 shrink-0">
              <Button asChild variant="secondary" size="sm">
                <Link href={`/learningpaths/${learningPath.id}/edit`}>Edit</Link>
              </Button>
              {deleteConfirm ? (
                <div className="flex gap-2 items-center">
                  <Button variant="destructive" size="sm" onClick={handleDelete}>
                    Confirm delete
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setDeleteConfirm(false)}>
                    Cancel
                  </Button>
                </div>
              ) : (
                <Button variant="destructive" size="sm" onClick={() => setDeleteConfirm(true)}>
                  Delete
                </Button>
              )}
            </div>
          </div>

          {learningPath.goal && <p className="text-subtle mb-2">{learningPath.goal}</p>}
          {learningPath.routine && (
            <p className="text-sm text-muted mb-4">Routine: {learningPath.routine}</p>
          )}
          {totalItems > 0 && (
            <p className="text-sm text-muted mb-6">
              {completedItems}/{totalItems} items complete
            </p>
          )}

          {learningPath.modules.length > 0 && (
            <section className="mb-6 space-y-4">
              {learningPath.modules.map((module) => (
                <div
                  key={module.id}
                  className="rounded-2xl border border-border bg-surface/50 overflow-hidden"
                >
                  <h2 className="bg-surface px-4 py-2 text-lg font-semibold border-b border-border">
                    {module.title}
                  </h2>
                  <ul className="px-4 py-2">
                    {module.items.map((item) => (
                      <li
                        key={item.id}
                        className="flex items-start gap-2 py-2 border-b last:border-0 border-border"
                      >
                        <Checkbox
                          checked={item.completed}
                          onChange={(e) => handleToggleItem(item.id, e.target.checked)}
                          aria-label={item.description}
                        />
                        <div>
                          {item.type && (
                            <span className="text-xs uppercase tracking-wide text-muted mr-2">
                              {item.type}
                            </span>
                          )}
                          <span className={item.completed ? 'line-through text-muted' : undefined}>
                            {item.description}
                          </span>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </section>
          )}

          {learningPath.resources.length > 0 && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Resources</h2>
              <ul className="list-disc list-inside space-y-1">
                {learningPath.resources.map((resource) => (
                  <li key={resource.id}>{resource.text}</li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </PageContainer>
  )
}
