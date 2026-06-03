'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import { Button } from '@/components/ui/button'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

export function SettingsWorkspaces({ data, mutate }: Props) {
  const [name, setName] = useState('')
  const [adding, setAdding] = useState(false)

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    const client = createServiceClient(SettingsService)
    await client.addWorkspace({ name: name.trim() })
    setName('')
    setAdding(false)
    mutate()
  }

  async function handleDelete(id: string) {
    const client = createServiceClient(SettingsService)
    await client.deleteWorkspace({ id })
    mutate()
  }

  return (
    <section aria-labelledby="workspaces-heading">
      <h2
        id="workspaces-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        Workspaces
      </h2>
      {data.workspaces.length === 0 ? (
        <p className="text-sm text-muted">No workspaces.</p>
      ) : (
        <ul className="mb-3 space-y-1">
          {data.workspaces.map((ws) => (
            <li
              key={ws.id}
              className="flex items-center justify-between rounded border border-border bg-card px-3 py-2"
            >
              <span className="text-sm text-subtle">{ws.name}</span>
              <button
                type="button"
                onClick={() => handleDelete(ws.id)}
                className="text-xs text-danger hover:underline"
              >
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex gap-2">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Workspace name"
            autoFocus
            className="flex-1 rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
          />
          <Button type="submit" size="sm" disabled={!name.trim()}>
            Add
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={() => setAdding(false)}>
            Cancel
          </Button>
        </form>
      ) : (
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="text-sm text-accent hover:underline"
        >
          + Add workspace
        </button>
      )}
    </section>
  )
}
