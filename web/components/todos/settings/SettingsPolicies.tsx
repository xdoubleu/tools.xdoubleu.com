'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse, Policy } from '@/lib/gen/todos/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

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
    await client.createPolicy({
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
    await client.deletePolicy({ id })
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
              <li key={policy.id} className="rounded-xl border border-warn/30 bg-warn/10 p-3">
                {edit ? (
                  <div className="flex flex-col gap-2">
                    <Textarea
                      value={edit.text}
                      onChange={(e) =>
                        setEditing((prev) => ({
                          ...prev,
                          [policy.id]: { ...prev[policy.id], text: e.target.value }
                        }))
                      }
                      rows={2}
                    />
                    <div className="flex items-center gap-2">
                      <label className="text-xs text-muted">Re-appear after</label>
                      <Input
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
                        className="h-9 w-16 px-2"
                      />
                      <label className="text-xs text-muted">hours</label>
                    </div>
                    <div className="flex gap-2">
                      <Button type="button" size="sm" onClick={() => handleUpdate(policy.id)}>
                        Save
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          setEditing((prev) => {
                            const next = { ...prev }
                            delete next[policy.id]
                            return next
                          })
                        }
                      >
                        Cancel
                      </Button>
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
                      <Button
                        type="button"
                        variant="link"
                        size="sm"
                        onClick={() => startEdit(policy)}
                        className="h-auto px-0 text-xs"
                      >
                        Edit
                      </Button>
                      <Button
                        type="button"
                        variant="link"
                        size="sm"
                        onClick={() => handleDelete(policy.id)}
                        className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
                      >
                        Delete
                      </Button>
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
          <Textarea
            value={addText}
            onChange={(e) => setAddText(e.target.value)}
            placeholder="Policy text"
            rows={2}
            autoFocus
          />
          <div className="flex items-center gap-2">
            <label className="text-xs text-muted">Re-appear after</label>
            <Input
              type="number"
              min={0}
              value={addHours}
              onChange={(e) => setAddHours(Number(e.target.value))}
              className="h-9 w-16 px-2"
            />
            <label className="text-xs text-muted">hours</label>
          </div>
          {data.workspaces.length > 0 && (
            <Select value={addWorkspaceId} onChange={(e) => setAddWorkspaceId(e.target.value)}>
              <option value="">No workspace</option>
              {data.workspaces.map((ws) => (
                <option key={ws.id} value={ws.id}>
                  {ws.name}
                </option>
              ))}
            </Select>
          )}
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={!addText.trim()}>
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
          + Add policy
        </Button>
      )}
    </section>
  )
}
