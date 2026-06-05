'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

const emptyForm = { urlPrefix: '', platformName: '', label: '', shortcut: '' }

export function SettingsURLPatterns({ data, mutate }: Props) {
  const [form, setForm] = useState(emptyForm)
  const [adding, setAdding] = useState(false)

  function setField(key: keyof typeof emptyForm, val: string) {
    setForm((prev) => ({ ...prev, [key]: val }))
  }

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!form.urlPrefix.trim() || !form.platformName.trim()) return
    const client = createServiceClient(SettingsService)
    await client.addURLPattern({
      urlPrefix: form.urlPrefix.trim(),
      platformName: form.platformName.trim(),
      label: form.label.trim(),
      shortcut: form.shortcut.trim(),
      workspaceId: ''
    })
    setForm(emptyForm)
    setAdding(false)
    mutate()
  }

  async function handleDelete(id: string) {
    const client = createServiceClient(SettingsService)
    await client.removeURLPattern({ id })
    mutate()
  }

  return (
    <section aria-labelledby="patterns-heading">
      <h2
        id="patterns-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        URL Patterns
      </h2>
      {data.urlPatterns.length === 0 ? (
        <p className="mb-3 text-sm text-muted">No URL patterns.</p>
      ) : (
        <ul className="mb-3 space-y-2">
          {data.urlPatterns.map((pattern) => (
            <li key={pattern.id} className="rounded-xl border border-border bg-card p-3">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <p className="text-sm font-medium text-subtle">{pattern.platformName}</p>
                  <p className="text-xs text-muted">{pattern.urlPrefix}</p>
                  {pattern.label && (
                    <span className="mt-1 inline-block rounded-full bg-surface px-1.5 py-0.5 text-xs text-muted">
                      {pattern.label}
                    </span>
                  )}
                  {pattern.shortcut && (
                    <span className="ml-1 mt-1 inline-block rounded-full bg-surface px-1.5 py-0.5 text-xs text-muted">
                      {pattern.shortcut}
                    </span>
                  )}
                </div>
                <Button
                  type="button"
                  variant="link"
                  size="sm"
                  onClick={() => handleDelete(pattern.id)}
                  className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
                >
                  Delete
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex flex-col gap-2">
          <Input
            type="text"
            value={form.platformName}
            onChange={(e) => setField('platformName', e.target.value)}
            placeholder="Platform name (e.g. GitHub)"
            autoFocus
          />
          <Input
            type="text"
            value={form.urlPrefix}
            onChange={(e) => setField('urlPrefix', e.target.value)}
            placeholder="URL prefix (e.g. https://github.com/)"
          />
          <Input
            type="text"
            value={form.label}
            onChange={(e) => setField('label', e.target.value)}
            placeholder="Label (optional)"
          />
          <Input
            type="text"
            value={form.shortcut}
            onChange={(e) => setField('shortcut', e.target.value)}
            placeholder="Shortcut key (optional)"
          />
          <div className="flex gap-2">
            <Button
              type="submit"
              size="sm"
              disabled={!form.urlPrefix.trim() || !form.platformName.trim()}
            >
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
          + Add URL pattern
        </Button>
      )}
    </section>
  )
}
