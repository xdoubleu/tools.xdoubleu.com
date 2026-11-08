import { useState } from 'react'
import { Link } from '@/components/ui/link'
import { useRouter } from '@/hooks/navigation'
import {
  useLearningPath,
  useDeleteLearningPath,
  useRecordItemProgress,
  useSetLearningPathPaused
} from '@/hooks/useLearningPaths'
import type { DeleteLearningPathInput } from '@/hooks/useLearningPaths'
import { useTodoistConnection } from '@/hooks/useTodoistConnection'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmDialog } from '@/components/ui/dialog'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'
import { ProgressBar } from '@/components/ui/progress-bar'
import { SectionHeading } from '@/components/ui/section-heading'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { cn } from '@/lib/cn'
import { isQuizPassed, moduleQuiz } from '@/lib/learningpaths/quiz'

export default function PathClient({ id }: { id: string }) {
  const { data, error, isLoading, mutate } = useLearningPath(id)
  const deleteLearningPath = useDeleteLearningPath()
  const recordItemProgress = useRecordItemProgress()
  const setLearningPathPaused = useSetLearningPathPaused()
  const { data: todoistStatus } = useTodoistConnection()
  const router = useRouter()

  const [deleteConfirm, setDeleteConfirm] = useState(false)
  const [pausing, setPausing] = useState(false)

  const learningPath = data?.learningPath

  const handleDelete = async () => {
    if (!learningPath) return
    const req: DeleteLearningPathInput = { id: learningPath.id }
    await deleteLearningPath(req)
    router.push('/learningpaths/list')
  }

  const handleTogglePaused = async () => {
    if (!learningPath) return
    setPausing(true)
    try {
      await setLearningPathPaused({ id: learningPath.id, paused: !learningPath.paused })
      await mutate()
    } finally {
      setPausing(false)
    }
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
              {(!todoistStatus?.connected || todoistStatus.needsReconnect) && (
                <PageHeaderLink href="/learningpaths/settings">
                  {todoistStatus?.needsReconnect ? 'Reconnect Todoist' : 'Connect Todoist'}
                </PageHeaderLink>
              )}
              <Button variant="secondary" onClick={handleTogglePaused} disabled={pausing}>
                {learningPath.paused ? 'Resume' : 'Pause'}
              </Button>
              <PageHeaderLink href={`/learningpaths/${learningPath.id}/edit`}>Edit</PageHeaderLink>
              <Button variant="destructive" onClick={() => setDeleteConfirm(true)}>
                Delete
              </Button>
            </>
          )
        }
      />
      {learningPath && (
        <ConfirmDialog
          open={deleteConfirm}
          onOpenChange={setDeleteConfirm}
          title={`Delete ${learningPath.title}?`}
          description="Its modules and progress are deleted. This can't be undone."
          confirmLabel="Delete"
          destructive
          onConfirm={handleDelete}
        />
      )}

      {isLoading && !learningPath && <LoadingState placement="page" label="learning path" />}
      {error && <ErrorState placement="page" what="learning path" />}
      {learningPath && (
        <>
          {learningPath.goal && <p className="text-subtle mb-2">{learningPath.goal}</p>}
          {learningPath.routine && (
            <p className="text-sm text-muted mb-4">Routine: {learningPath.routine}</p>
          )}
          {learningPath.paused ? (
            <p className="text-sm text-muted mb-4">
              This path is paused, so it has no Todoist reminders. Resume it to get them again.
            </p>
          ) : (
            todoistStatus?.connected && (
              <p className="text-sm text-muted mb-4">
                {todoistStatus.needsReconnect
                  ? 'Todoist reminders are paused until you reconnect Todoist.'
                  : 'Reminders for the current week are created automatically in your Todoist project.'}
              </p>
            )
          )}
          {totalItems > 0 && (
            <div className="mb-6 space-y-1">
              <ProgressBar
                value={completedItems}
                max={totalItems}
                aria-label="Path progress"
                valueText={`${completedItems} of ${totalItems} items`}
              />
              <p className="text-sm text-muted">
                {completedItems}/{totalItems} items complete
              </p>
            </div>
          )}

          {learningPath.modules.length > 0 && (
            <section className="mb-6 space-y-4">
              {learningPath.modules.map((module) => (
                <Card key={module.id} variant="inset">
                  <SectionHeading className="mb-2">{module.title}</SectionHeading>
                  <ul>
                    {module.items.map((item) => (
                      <li
                        key={item.id}
                        className="flex flex-wrap items-center gap-x-2 border-b border-border py-1 last:border-0"
                      >
                        {item.linkedBook ? (
                          <Badge className="max-w-full flex-wrap gap-1.5 self-start">
                            <span className="min-w-0 break-words">
                              {item.type && (
                                <span className="mr-2 text-xs uppercase tracking-wide text-muted">
                                  {item.type}
                                </span>
                              )}
                              {item.description}
                            </span>
                            <span className="text-muted">
                              — 📚 {item.linkedBook.title}
                              {item.linkedBook.progressPercent > 0 &&
                                `, ${item.linkedBook.progressPercent}%`}
                            </span>
                          </Badge>
                        ) : (
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
                        )}
                      </li>
                    ))}
                  </ul>
                  {moduleQuiz(module) && (
                    <div className="mt-3">
                      {isQuizPassed(module) ? (
                        <Badge variant="success">Quiz passed</Badge>
                      ) : (
                        <Button asChild size="sm">
                          <Link href={`/learningpaths/${learningPath.id}/quiz/${module.id}`}>
                            Take quiz ({module.quiz.length} questions)
                          </Link>
                        </Button>
                      )}
                    </div>
                  )}
                </Card>
              ))}
            </section>
          )}

          {learningPath.resources.length > 0 && (
            <section className="mb-6">
              <SectionHeading className="mb-3">Resources</SectionHeading>
              <ul className="space-y-2">
                {learningPath.resources.map((resource) => (
                  <li key={resource.id} className="flex flex-col gap-1">
                    {resource.text && <span className="break-words">{resource.text}</span>}
                    {resource.linkedBook && (
                      <Badge className="max-w-full flex-wrap gap-1.5 self-start">
                        <span className="break-words">📚 {resource.linkedBook.title}</span>
                        <span className="text-muted">
                          — {resource.linkedBook.status.replace('_', ' ')}
                          {resource.linkedBook.progressPercent > 0 &&
                            `, ${resource.linkedBook.progressPercent}%`}
                        </span>
                      </Badge>
                    )}
                    {resource.linkedFeedItem && (
                      <Badge className="max-w-full flex-wrap gap-1.5 self-start">
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
