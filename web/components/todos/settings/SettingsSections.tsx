'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

export function SettingsSections({ data, mutate }: Props) {
  const [name, setName] = useState('')
  const [workspaceId, setWorkspaceId] = useState('')
  const [adding, setAdding] = useState(false)

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    const client = createServiceClient(SettingsService)
    await client.addSection({ name: name.trim(), workspaceId })
    setName('')
    setAdding(false)
    mutate()
  }

  async function handleDelete(id: string) {
    const client = createServiceClient(SettingsService)
    await client.removeSection({ id })
    mutate()
  }

  return (
    <section aria-labelledby="sections-heading">
      <h2
        id="sections-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        Sections
      </h2>
      {data.sections.length === 0 ? (
        <p className="text-sm text-muted">No sections.</p>
      ) : (
        <ul className="mb-3 space-y-1">
          {data.sections.map((sec) => (
            <li
              key={sec.id}
              className="flex items-center justify-between rounded border border-border bg-card px-3 py-2"
            >
              <span className="text-sm text-subtle">{sec.name}</span>
              <button
                type="button"
                onClick={() => handleDelete(sec.id)}
                className="text-xs text-danger hover:underline"
              >
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex flex-col gap-2">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Section name"
            autoFocus
            className="rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
          />
          {data.workspaces.length > 0 && (
            <select
              value={workspaceId}
              onChange={(e) => setWorkspaceId(e.target.value)}
              className="rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
            >
              <option value="">No workspace</option>
              {data.workspaces.map((ws) => (
                <option key={ws.id} value={ws.id}>
                  {ws.name}
                </option>
              ))}
            </select>
          )}
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={!name.trim()}
              className="rounded bg-accent px-3 py-1.5 text-sm font-medium text-white hover:bg-accent-hover disabled:opacity-50"
            >
              Add
            </button>
            <button
              type="button"
              onClick={() => setAdding(false)}
              className="rounded px-3 py-1.5 text-sm text-muted hover:text-subtle"
            >
              Cancel
            </button>
          </div>
        </form>
      ) : (
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="text-sm text-accent hover:underline"
        >
          + Add section
        </button>
      )}
    </section>
  )
}
