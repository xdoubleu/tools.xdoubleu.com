import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import {
  TaskPriority,
  type Repo,
  type Task,
  type TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'
import { COLUMNS, priorityLabel, repoLabel } from '@/lib/factory/board'
import { useFactoryActions } from '@/hooks/useFactory'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Select } from '@/components/ui/select'

const PRIORITY_VARIANT: Partial<Record<TaskPriority, 'danger' | 'warn' | 'secondary'>> = {
  [TaskPriority.P0]: 'danger',
  [TaskPriority.P1]: 'warn',
  [TaskPriority.P2]: 'secondary'
}

interface TaskCardProps {
  task: Task
  /** Shown on the card when the board mixes several repos. */
  repo?: Repo
  onOpen: (id: string) => void
}

/** Select that moves a task between columns; shows a message when the move is refused. */
export function MoveSelect({ task }: { task: Task }) {
  const { move } = useFactoryActions()
  const [failure, setFailure] = useState('')

  const onChange = async (status: TaskStatus) => {
    setFailure('')
    try {
      await move(task.id, status)
    } catch (err) {
      setFailure(
        ConnectError.from(err).code === Code.PermissionDenied
          ? 'Only you, in the browser, can mark a task Ready.'
          : "Couldn't move the task. Try again."
      )
    }
  }

  return (
    <>
      <Select
        aria-label={`Move ${task.title}`}
        value={task.status}
        onChange={(e) => void onChange(Number(e.target.value))}
      >
        {COLUMNS.map((c) => (
          <option key={c.status} value={c.status}>
            {c.label}
          </option>
        ))}
      </Select>
      {failure && (
        <p className="text-xs text-danger" role="alert">
          {failure}
        </p>
      )}
    </>
  )
}

export default function TaskCard({ task, repo, onOpen }: TaskCardProps) {
  return (
    <Card variant="inset" className="space-y-2">
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
        {task.labels.map((l) => (
          <Badge key={l} variant="default">
            {l}
          </Badge>
        ))}
        {task.commentCount > 0 && (
          <span className="text-xs text-muted">{task.commentCount} comments</span>
        )}
      </div>
      <MoveSelect task={task} />
    </Card>
  )
}
