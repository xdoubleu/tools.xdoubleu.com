'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { TaskService } from '@/lib/gen/todos/v1/tasks_connect'
import { formatRelativeDate, isOverdue } from '@/lib/todos/dateUtils'
import type { Subtask } from '@/lib/gen/todos/v1/tasks_pb'

export default function TaskClient({ id }: { id: string }) {
  const router = useRouter()
  const client = createServiceClient(TaskService)

  const { data, isLoading, error } = useSWR(id ? `/todos/tasks/${id}` : null, () =>
    client.getTask({ id })
  )

  const [subtaskDone, setSubtaskDone] = useState<Record<string, boolean>>({})

  const task = data?.task ?? null

  async function handleAction(action: 'complete' | 'reopen' | 'delete') {
    const base = process.env.NEXT_PUBLIC_API_URL ?? ''
    await fetch(`${base}/todos/${id}/${action}`, { method: 'POST' })
    if (action === 'delete') {
      router.push('/todos')
    } else {
      router.refresh()
    }
  }

  function toggleSubtask(subtaskId: string) {
    setSubtaskDone((prev) => ({ ...prev, [subtaskId]: !prev[subtaskId] }))
  }

  function renderSubtasks(subtasks: Subtask[], depth = 0) {
    return subtasks.map((st) => (
      <li key={st.id} style={{ paddingLeft: `${depth * 16}px` }}>
        <label className="flex cursor-pointer items-start gap-2 py-1">
          <input
            type="checkbox"
            checked={subtaskDone[st.id] ?? st.done}
            onChange={() => toggleSubtask(st.id)}
            className="mt-0.5"
          />
          <span
            className={`text-sm ${(subtaskDone[st.id] ?? st.done) ? 'line-through text-muted' : 'text-subtle'}`}
          >
            {st.title}
          </span>
        </label>
        {st.children.length > 0 && <ul>{renderSubtasks(st.children, depth + 1)}</ul>}
      </li>
    ))
  }

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error || !task) {
    return <p className="py-16 text-center text-sm text-red-500">Task not found.</p>
  }

  const dueLabel = task.dueDate ? formatRelativeDate(task.dueDate) : null
  const deadlineLabel = task.deadline ? formatRelativeDate(task.deadline) : null
  const overdue = isOverdue(task.dueDate)

  return (
    <article className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-fg">{task.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted">
          {task.priority > 0 && (
            <span className="rounded bg-surface px-1.5 py-0.5 text-xs font-semibold text-muted">
              P{task.priority}
            </span>
          )}
          {task.labels.map((label) => (
            <span key={label} className="rounded bg-blue-50 px-1.5 py-0.5 text-xs text-blue-700 dark:bg-blue-900 dark:text-blue-300">
              {label}
            </span>
          ))}
          {dueLabel && (
            <span className={overdue ? 'font-semibold text-red-600 dark:text-red-400' : ''}>Due: {dueLabel}</span>
          )}
          {deadlineLabel && <span>Deadline: {deadlineLabel}</span>}
        </div>
      </div>

      {task.description && (
        <section aria-label="Description">
          <h2 className="mb-1 text-sm font-semibold text-subtle">Description</h2>
          <p className="text-sm text-muted">{task.description}</p>
        </section>
      )}

      {task.subtasks.length > 0 && (
        <section aria-label="Subtasks">
          <h2 className="mb-2 text-sm font-semibold text-subtle">
            Subtasks ({task.subtaskDone}/{task.subtaskTotal})
          </h2>
          <ul className="space-y-0.5">{renderSubtasks(task.subtasks)}</ul>
        </section>
      )}

      {task.links.length > 0 && (
        <section aria-label="Links">
          <h2 className="mb-2 text-sm font-semibold text-subtle">Links</h2>
          <ul className="space-y-1">
            {task.links.map((link) => (
              <li key={link.id}>
                <a
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-blue-600 hover:underline"
                >
                  {link.label || link.url}
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="flex gap-2 border-t border-border pt-4">
        {task.status === 'open' ? (
          <button
            type="button"
            onClick={() => handleAction('complete')}
            className="rounded bg-green-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-green-700"
          >
            Complete
          </button>
        ) : (
          <button
            type="button"
            onClick={() => handleAction('reopen')}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
          >
            Reopen
          </button>
        )}
        <button
          type="button"
          onClick={() => handleAction('delete')}
          className="rounded border border-red-300 px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50"
        >
          Delete
        </button>
      </div>
    </article>
  )
}
