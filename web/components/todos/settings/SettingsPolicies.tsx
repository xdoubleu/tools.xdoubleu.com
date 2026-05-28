'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse, Policy } from '@/lib/gen/todos/v1/settings_pb'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

interface EditState {
  text: string
  reappearAfterHours: number
}

export function SettingsPolicies({ data, mutate }: Props) {
  const [addText, setAddText] = useState('')
  const [addHours, setAddHours] = useState(24)
  const [addWorkspaceId, setAddWorkspaceId] = useState('')
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<Record<string, EditState>>({})

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!addText.trim()) return
    const client = createServiceClient(SettingsService)
    await client.addPolicy({
      text: addText.trim(),
      reappearAfterHours: addHours,
      workspaceId: addWorkspaceId
    })
    setAddText('')
    setAdding(false)
    mutate()
  }

  function startEdit(policy: Policy) {
    setEditing((prev) => ({
      ...prev,
      [policy.id]: { text: policy.text, reappearAfterHours: policy.reappearAfterHours }
    }))
  }

  async function handleUpdate(id: string) {
    const state = editing[id]
    if (!state) return
    const client = createServiceClient(SettingsService)
    await client.updatePolicy({
      id,
      text: state.text,
      reappearAfterHours: state.reappearAfterHours
    })
    setEditing((prev) => {
      const next = { ...prev }
      delete next[id]
      return next
    })
    mutate()
  }

  async function handleDelete(id: string) {
    const client = createServiceClient(SettingsService)
    await client.removePolicy({ id })
    mutate()
  }

  return (
    <section aria-labelledby="policies-heading">
      <h2
        id="policies-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        Policies
      </h2>
      {data.policies.length === 0 ? (
        <p className="mb-3 text-sm text-muted">No policies.</p>
      ) : (
        <ul className="mb-3 space-y-2">
          {data.policies.map((policy) => {
            const edit = editing[policy.id]
            return (
              <li key={policy.id} className="rounded border border-warn/30 bg-warn/10 p-3">
                {edit ? (
                  <div className="flex flex-col gap-2">
                    <textarea
                      value={edit.text}
                      onChange={(e) =>
                        setEditing((prev) => ({
                          ...prev,
                          [policy.id]: { ...prev[policy.id], text: e.target.value }
                        }))
                      }
                      rows={2}
                      className="rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
                    />
                    <div className="flex items-center gap-2">
                      <label className="text-xs text-muted">Re-appear after</label>
                      <input
                        type="number"
                        min={0}
                        value={edit.reappearAfterHours}
                        onChange={(e) =>
                          setEditing((prev) => ({
                            ...prev,
                            [policy.id]: {
                              ...prev[policy.id],
                              reappearAfterHours: Number(e.target.value)
                            }
                          }))
                        }
                        className="w-16 rounded border border-input-border bg-input px-2 py-1 text-sm text-input-text"
                      />
                      <label className="text-xs text-muted">hours</label>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => handleUpdate(policy.id)}
                        className="rounded bg-accent px-3 py-1 text-xs font-medium text-white hover:bg-accent-hover"
                      >
                        Save
                      </button>
                      <button
                        type="button"
                        onClick={() =>
                          setEditing((prev) => {
                            const next = { ...prev }
                            delete next[policy.id]
                            return next
                          })
                        }
                        className="rounded px-3 py-1 text-xs text-muted hover:text-subtle"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="text-sm text-fg">{policy.text}</p>
                      <p className="mt-1 text-xs text-muted">
                        Re-appears after {policy.reappearAfterHours}h
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => startEdit(policy)}
                        className="text-xs text-accent hover:underline"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDelete(policy.id)}
                        className="text-xs text-danger hover:underline"
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                )}
              </li>
            )
          })}
        </ul>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex flex-col gap-2">
          <textarea
            value={addText}
            onChange={(e) => setAddText(e.target.value)}
            placeholder="Policy text"
            rows={2}
            autoFocus
            className="rounded border border-input-border bg-input px-3 py-1.5 text-sm text-input-text"
          />
          <div className="flex items-center gap-2">
            <label className="text-xs text-muted">Re-appear after</label>
            <input
              type="number"
              min={0}
              value={addHours}
              onChange={(e) => setAddHours(Number(e.target.value))}
              className="w-16 rounded border border-input-border bg-input px-2 py-1 text-sm text-input-text"
            />
            <label className="text-xs text-muted">hours</label>
          </div>
          {data.workspaces.length > 0 && (
            <select
              value={addWorkspaceId}
              onChange={(e) => setAddWorkspaceId(e.target.value)}
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
              disabled={!addText.trim()}
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
          + Add policy
        </button>
      )}
    </section>
  )
}
