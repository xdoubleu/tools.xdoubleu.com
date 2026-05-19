import type { Task } from '@/lib/gen/todos/v1/tasks_pb'
import { formatRelativeDate, isOverdue } from '@/lib/todos/dateUtils'

const PRIORITY_LABELS: Record<number, string> = {
  1: 'P1',
  2: 'P2',
  3: 'P3',
  4: 'P4'
}

const PRIORITY_CLASSES: Record<number, string> = {
  1: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  2: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  3: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300',
  4: 'bg-surface text-muted'
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
      className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-card p-3 shadow-sm hover:bg-surface"
      onClick={onClick}
    >
      {/* Priority badge */}
      {task.priority > 0 && (
        <span
          className={`mt-0.5 rounded px-1.5 py-0.5 text-xs font-semibold ${PRIORITY_CLASSES[task.priority] ?? 'bg-surface text-muted'}`}
        >
          {PRIORITY_LABELS[task.priority] ?? `P${task.priority}`}
        </span>
      )}

      <div className="min-w-0 flex-1">
        {/* Title */}
        <p
          className={`truncate text-sm font-medium ${task.status === 'done' ? 'line-through text-muted' : 'text-fg'}`}
        >
          {task.title}
        </p>

        {/* Labels and due date row */}
        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          {task.labels.map((label) => (
            <span key={label} className="rounded bg-blue-50 px-1.5 py-0.5 text-xs text-blue-700 dark:bg-blue-900 dark:text-blue-300">
              {label}
            </span>
          ))}
          {dueLabel && (
            <span className={`text-xs ${overdue ? 'font-semibold text-red-600' : 'text-muted'}`}>
              {dueLabel}
            </span>
          )}
        </div>

        {/* Subtask progress */}
        {task.subtaskTotal > 0 && (
          <p className="mt-1 text-xs text-muted">
            {task.subtaskDone}/{task.subtaskTotal} subtasks
          </p>
        )}
      </div>
    </div>
  )
}
