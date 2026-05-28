import type { Task } from '@/lib/gen/todos/v1/tasks_pb'
import { formatRelativeDate, isOverdue } from '@/lib/todos/dateUtils'

const PRIORITY_LABELS: Record<number, string> = {
  1: 'P1',
  2: 'P2',
  3: 'P3',
  4: 'P4'
}

const PRIORITY_CLASSES: Record<number, string> = {
  1: 'bg-danger/10 text-danger border-danger/20',
  2: 'bg-warn/10 text-warn border-warn/20',
  3: 'bg-warn/10 text-warn border-warn/20',
  4: 'bg-surface text-muted border-border'
}

interface TaskCardProps {
  task: Task
  onClick?: () => void
}

export function TaskCard({ task, onClick }: TaskCardProps) {
  const dueLabel = task.dueDate ? formatRelativeDate(task.dueDate) : null
  const overdue = isOverdue(task.dueDate)

  return (
    <div
      role="listitem"
      className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-card p-3 shadow-card transition-shadow hover:shadow-elevated active:scale-[0.99]"
      onClick={onClick}
    >
      {task.priority > 0 && (
        <span
          className={`mt-0.5 rounded-lg border px-1.5 py-0.5 text-xs font-semibold ${PRIORITY_CLASSES[task.priority] ?? 'bg-surface text-muted border-border'}`}
        >
          {PRIORITY_LABELS[task.priority] ?? `P${task.priority}`}
        </span>
      )}

      <div className="min-w-0 flex-1">
        <p
          className={`truncate text-sm font-medium ${task.status === 'done' ? 'line-through text-muted' : 'text-fg'}`}
        >
          {task.title}
        </p>

        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          {task.labels.map((label) => (
            <span
              key={label}
              className="rounded-full bg-accent/10 px-2 py-0.5 text-xs text-accent border border-accent/20"
            >
              {label}
            </span>
          ))}
          {dueLabel && (
            <span className={`text-xs ${overdue ? 'font-semibold text-danger' : 'text-muted'}`}>
              {dueLabel}
            </span>
          )}
        </div>

        {task.subtaskTotal > 0 && (
          <p className="mt-1 text-xs text-muted">
            {task.subtaskDone}/{task.subtaskTotal} subtasks
          </p>
        )}
      </div>
    </div>
  )
}
