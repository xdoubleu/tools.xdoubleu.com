'use client'

import type { Task } from '@/lib/gen/todos/v1/tasks_pb'
import { SubtaskService } from '@/lib/gen/todos/v1/subtasks_pb'
import { formatRelativeDate, isOverdue } from '@/lib/todos/dateUtils'
import { createServiceClient } from '@/lib/client'

const PRIORITY_LABELS: Record<number, string> = {
  1: 'P1',
  2: 'P2',
  3: 'P3',
  4: 'P4'
}

const PRIORITY_CLASSES: Record<number, string> = {
  1: 'bg-danger/10 text-danger border-danger/20',
  2: 'bg-warn/10 text-fg border-warn/20',
  3: 'bg-warn/10 text-fg border-warn/20',
  4: 'bg-surface text-muted border-border'
}

interface TaskCardProps {
  task: Task
  onClick?: () => void
  onChanged?: () => void
}

export function TaskCard({ task, onClick, onChanged }: TaskCardProps) {
  const dueLabel = task.dueDate ? formatRelativeDate(task.dueDate) : null
  const overdue = isOverdue(task.dueDate)

  const topLevelSubtasks = task.subtasks.filter((s) => s.parentSubtaskId === '')

  async function handleSubtaskToggle(e: React.MouseEvent, subtaskId: string) {
    e.stopPropagation()
    const client = createServiceClient(SubtaskService)
    await client.toggleSubtask({ subtaskId, taskId: task.id })
    onChanged?.()
  }

  return (
    <div
      role="listitem"
      className="cursor-pointer rounded-xl border border-border bg-card p-3 shadow-card transition-shadow hover:shadow-elevated active:scale-[0.99]"
      onClick={onClick}
    >
      <div className="flex items-start gap-3">
        {task.priority > 0 && (
          <span
            className={`mt-0.5 shrink-0 rounded-lg border px-1.5 py-0.5 text-xs font-semibold ${PRIORITY_CLASSES[task.priority] ?? 'bg-surface text-muted border-border'}`}
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
                className="rounded-full border border-accent/20 bg-accent/10 px-2 py-0.5 text-xs text-accent"
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
        </div>
      </div>

      {topLevelSubtasks.length > 0 && (
        <ul className="mt-2 space-y-1 border-t border-border pt-2">
          {topLevelSubtasks.map((sub) => (
            <li key={sub.id} className="flex items-center gap-2">
              <button
                type="button"
                aria-label={sub.done ? 'Mark incomplete' : 'Mark done'}
                onClick={(e) => handleSubtaskToggle(e, sub.id)}
                className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors ${sub.done ? 'border-accent bg-accent text-white' : 'border-border bg-card hover:border-accent'}`}
              >
                {sub.done && (
                  <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
                    <path
                      d="M2 5l2.5 2.5L8 3"
                      stroke="currentColor"
                      strokeWidth="1.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                )}
              </button>
              <span
                className={`truncate text-xs ${sub.done ? 'text-muted line-through' : 'text-subtle'}`}
                title={sub.title}
              >
                {sub.title.length > 60 ? `${sub.title.slice(0, 60)}…` : sub.title}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
