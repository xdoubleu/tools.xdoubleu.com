'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'

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
              className="flex items-center justify-between rounded-xl border border-border bg-card px-3 py-2"
            >
              <span className="text-sm text-subtle">{sec.name}</span>
              <Button
                type="button"
                variant="link"
                size="sm"
                onClick={() => handleDelete(sec.id)}
                className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
              >
                Delete
              </Button>
            </li>
          ))}
        </ul>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex flex-col gap-2">
          <Input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Section name"
            autoFocus
          />
          {data.workspaces.length > 0 && (
            <Select value={workspaceId} onChange={(e) => setWorkspaceId(e.target.value)}>
              <option value="">No workspace</option>
              {data.workspaces.map((ws) => (
                <option key={ws.id} value={ws.id}>
                  {ws.name}
                </option>
              ))}
            </Select>
          )}
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={!name.trim()}>
              Add
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setAdding(false)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : (
        <Button
          type="button"
          variant="link"
          size="sm"
          className="self-start"
          onClick={() => setAdding(true)}
        >
          + Add section
        </Button>
      )}
    </section>
  )
}
