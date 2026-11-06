import { type DragEvent } from 'react'
import { TaskPriority, type Repo, type Task } from '@/lib/gen/factory/v1/factory_pb'
import { prBadge, priorityLabel, repoLabel } from '@/lib/factory/board'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
  onOpen: (id: string) => void
  onDragStart: (event: DragEvent<HTMLDivElement>, task: Task) => void
  onDragEnd: () => void
}

export default function TaskCard({
  task,
  repo,
  awaitingApproval,
  dragging,
  onOpen,
  onDragStart,
  onDragEnd
}: TaskCardProps) {
  const pr = prBadge(task)
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
      {repo && <p className="break-words text-xs text-muted">{repoLabel(repo)}</p>}
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
          <Badge key={l} variant="default">
            {l}
          </Badge>
        ))}
        {task.commentCount > 0 && (
          <span className="text-xs text-muted">{task.commentCount} comments</span>
        )}
      </div>
      {awaitingApproval && (
        <p className="text-xs text-warn" data-testid="approval-hint">
          Not approved for auto pickup. Edit and save its text in the app to approve it.
        </p>
      )}
    </Card>
  )
}

export type { TaskCardProps }
