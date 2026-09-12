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
import { useTodoistConnection, useSendItemToTodoist } from '@/hooks/useTodoistConnection'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function PathClient({ id }: { id: string }) {
  const { data, error, isLoading, mutate } = useLearningPath(id)
  const deleteLearningPath = useDeleteLearningPath()
  const recordItemProgress = useRecordItemProgress()
  const { data: todoistStatus } = useTodoistConnection()
  const sendItemToTodoist = useSendItemToTodoist()
  const router = useRouter()

  const [deleteConfirm, setDeleteConfirm] = useState(false)
  const [sendingItemId, setSendingItemId] = useState<string | null>(null)
  const [sentItemIds, setSentItemIds] = useState<Set<string>>(new Set())

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

  const handleSendToTodoist = async (itemId: string) => {
    setSendingItemId(itemId)
    try {
      await sendItemToTodoist(itemId)
      setSentItemIds((prev) => new Set(prev).add(itemId))
    } finally {
      setSendingItemId(null)
    }
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
          <div className="flex flex-wrap items-start justify-between gap-2 mb-2">
            <h1 className="text-3xl font-bold min-w-0 break-words">{learningPath.title}</h1>
            <div className="flex flex-wrap justify-end gap-2">
              {!todoistStatus?.connected && (
                <Button asChild variant="secondary" size="sm">
                  <Link href="/learningpaths/settings">Connect Todoist</Link>
                </Button>
              )}
              <Button asChild variant="secondary" size="sm">
                <Link href={`/learningpaths/${learningPath.id}/edit`}>Edit</Link>
              </Button>
              {deleteConfirm ? (
                <div className="flex flex-wrap gap-2 items-center">
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
                        className="flex flex-wrap items-start gap-2 py-2 border-b last:border-0 border-border"
                      >
                        <Checkbox
                          checked={item.completed}
                          onChange={(e) => handleToggleItem(item.id, e.target.checked)}
                          aria-label={item.description}
                        />
                        <div className="flex-1 min-w-0">
                          {item.type && (
                            <span className="text-xs uppercase tracking-wide text-muted mr-2">
                              {item.type}
                            </span>
                          )}
                          <span
                            className={`break-words ${item.completed ? 'line-through text-muted' : ''}`}
                          >
                            {item.description}
                          </span>
                        </div>
                        {todoistStatus?.connected && (
                          <Button
                            variant="secondary"
                            size="sm"
                            className="shrink-0"
                            disabled={sendingItemId === item.id || sentItemIds.has(item.id)}
                            onClick={() => handleSendToTodoist(item.id)}
                          >
                            {sentItemIds.has(item.id)
                              ? 'Sent'
                              : sendingItemId === item.id
                                ? 'Sending…'
                                : 'Send to Todoist'}
                          </Button>
                        )}
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
