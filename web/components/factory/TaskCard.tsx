import { useState, type DragEvent } from 'react'
import { TaskPriority, TaskStatus, type Repo, type Task } from '@/lib/gen/factory/v1/factory_pb'
import { isProductLabel, prBadge, priorityLabel, repoLabel, shortTaskId } from '@/lib/factory/board'
import { useFactoryActions } from '@/hooks/useFactory'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { FieldError } from '@/components/ui/field-error'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/cn'

const PRIORITY_VARIANT: Partial<Record<TaskPriority, 'danger' | 'warn' | 'secondary'>> = {
  [TaskPriority.P0]: 'danger',
  [TaskPriority.P1]: 'warn',
  [TaskPriority.P2]: 'secondary'
}

interface TaskCardProps {
  task: Task
  /** Shown on the card when the board mixes several repos. */
  repo?: Repo
  /** A Ready task of a repo with auto pickup whose text no human has approved yet. */
  awaitingApproval?: boolean
  /** Set by the board while this task is being dragged, to dim the source card. */
  dragging?: boolean
  /** Set when this is an epic whose sub-tasks are all Done: closes the epic. */
  onCloseEpic?: () => void
  onOpen: (id: string) => void
  onDragStart: (event: DragEvent<HTMLDivElement>, task: Task) => void
  onDragEnd: () => void
}

export default function TaskCard({
  task,
  repo,
  awaitingApproval,
  dragging,
  onCloseEpic,
  onOpen,
  onDragStart,
  onDragEnd
}: TaskCardProps) {
  const pr = prBadge(task)
  const { move } = useFactoryActions()
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState('')

  // Retry pickup is a fresh Ready move: it clears the block reason server-side.
  const retryPickup = async () => {
    setRetryError('')
    setRetrying(true)
    try {
      await move(task.id, TaskStatus.READY)
    } catch {
      setRetryError('Couldn\u2019t retry pickup. Try again.')
    } finally {
      setRetrying(false)
    }
  }

  return (
    <Card
      variant="inset"
      className={cn('space-y-2', dragging && 'opacity-40')}
      draggable
      onDragStart={(e) => onDragStart(e, task)}
      onDragEnd={onDragEnd}
    >
      <Button
        variant="link"
        className="h-auto w-full justify-start whitespace-normal p-0 text-left font-medium"
        onClick={() => onOpen(task.id)}
      >
        {task.title}
      </Button>
      <p className="break-words text-xs text-muted">
        <span data-testid="task-id">{shortTaskId(task.id)}</span>
        {repo && (
          <>
            {' · '}
            <span>{repoLabel(repo)}</span>
          </>
        )}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={PRIORITY_VARIANT[task.priority] ?? 'secondary'}>
          {priorityLabel(task.priority)}
        </Badge>
        {pr && (
          <Badge variant={pr.variant} data-testid="pr-badge">
            {pr.label}
          </Badge>
        )}
        {task.labels.map((l) => (
          <Badge
            key={l}
            variant={isProductLabel(l) ? 'secondary' : 'default'}
            data-testid={isProductLabel(l) ? 'product-label' : undefined}
          >
            {l}
          </Badge>
        ))}
        {task.commentCount > 0 && (
          <span className="text-xs text-muted">{task.commentCount} comments</span>
        )}
      </div>
      {onCloseEpic && (
        <Button variant="secondary" size="sm" onClick={onCloseEpic}>
          Close epic
        </Button>
      )}
      {awaitingApproval && (
        <p className="text-xs text-warn" data-testid="approval-hint">
          Not approved for auto pickup. Edit and save its text in the app to approve it.
        </p>
      )}
      {task.pickupBlockReason && (
        <div className="space-y-2">
          <p className="text-xs text-warn" data-testid="pickup-block">
            Not picked up: {task.pickupBlockReason}
          </p>
          <Button
            variant="secondary"
            size="sm"
            disabled={retrying}
            onClick={() => void retryPickup()}
          >
            {retrying ? 'Retrying…' : 'Retry pickup'}
          </Button>
          <FieldError>{retryError}</FieldError>
        </div>
      )}
    </Card>
  )
}

export type { TaskCardProps }
