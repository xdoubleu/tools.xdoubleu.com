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
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { cn } from '@/lib/cn'

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
    <PageContainer>
      <PageHeader
        className="mb-2"
        title={learningPath?.title ?? 'Learning Path'}
        breadcrumb={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: learningPath?.title ?? 'Learning Path' }
        ]}
        actions={
          learningPath && (
            <>
              {!todoistStatus?.connected && (
                <Button asChild variant="secondary" size="sm">
                  <Link href="/learningpaths/settings">Connect Todoist</Link>
                </Button>
              )}
              <Button asChild variant="secondary" size="sm">
                <Link href={`/learningpaths/${learningPath.id}/edit`}>Edit</Link>
              </Button>
              {deleteConfirm ? (
                <>
                  <Button variant="destructive" size="sm" onClick={handleDelete}>
                    Confirm delete
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setDeleteConfirm(false)}>
                    Cancel
                  </Button>
                </>
              ) : (
                <Button variant="destructive" size="sm" onClick={() => setDeleteConfirm(true)}>
                  Delete
                </Button>
              )}
            </>
          )
        }
      />

      {isLoading && !learningPath && <LoadingState label="learning path" />}
      {error && <ErrorState what="learning path" />}
      {learningPath && (
        <>
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
                <Card key={module.id} variant="inset">
                  <h2 className="text-lg font-semibold">{module.title}</h2>
                  <ul>
                    {module.items.map((item) => (
                      <li
                        key={item.id}
                        className="flex flex-wrap items-center gap-x-2 border-b border-border py-1 last:border-0"
                      >
                        <Checkbox
                          checked={item.completed}
                          onChange={(e) => handleToggleItem(item.id, e.target.checked)}
                          labelClassName="min-w-0 flex-1"
                          label={
                            <span className="min-w-0 break-words">
                              {item.type && (
                                <span className="mr-2 text-xs uppercase tracking-wide text-muted">
                                  {item.type}
                                </span>
                              )}
                              <span className={cn(item.completed && 'text-muted line-through')}>
                                {item.description}
                              </span>
                            </span>
                          }
                        />
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
                </Card>
              ))}
            </section>
          )}

          {learningPath.resources.length > 0 && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Resources</h2>
              <ul className="space-y-2">
                {learningPath.resources.map((resource) => (
                  <li key={resource.id} className="flex flex-col gap-1">
                    {resource.text && <span className="break-words">{resource.text}</span>}
                    {resource.linkedBook && (
                      <Badge className="max-w-full flex-wrap gap-1.5 self-start text-sm">
                        <span className="break-words">📚 {resource.linkedBook.title}</span>
                        <span className="text-muted">
                          — {resource.linkedBook.status.replace('_', ' ')}
                          {resource.linkedBook.progressPercent > 0 &&
                            `, ${resource.linkedBook.progressPercent}%`}
                        </span>
                      </Badge>
                    )}
                    {resource.linkedFeedItem && (
                      <Badge className="max-w-full flex-wrap gap-1.5 self-start text-sm">
                        <span className="break-words">📰 {resource.linkedFeedItem.title}</span>
                        <span className="text-muted">
                          — {resource.linkedFeedItem.read ? 'read' : 'unread'}
                        </span>
                      </Badge>
                    )}
                    {((resource.linkedBookId && !resource.linkedBook) ||
                      (resource.linkedFeedItemId && !resource.linkedFeedItem)) && (
                      <span className="text-muted italic">Linked resource no longer available</span>
                    )}
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </PageContainer>
  )
}
