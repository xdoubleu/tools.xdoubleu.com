'use client'

import { useState } from 'react'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import SettingsLabelRow from '@/components/todos/SettingsLabelRow'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const LABEL_CATEGORY = 'label'

interface Props {
  data: GetSettingsResponse
  mutate: () => void
}

export function SettingsLabels({ data, mutate }: Props) {
  const [value, setValue] = useState('')
  const [color, setColor] = useState('#6366f1')
  const [adding, setAdding] = useState(false)

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    if (!value.trim()) return
    const client = createServiceClient(SettingsService)
    await client.addLabelPreset({ category: LABEL_CATEGORY, value: value.trim(), workspaceId: '' })
    await client.updateLabelColor({
      category: LABEL_CATEGORY,
      value: value.trim(),
      color,
      workspaceId: ''
    })
    setValue('')
    setAdding(false)
    mutate()
  }

  async function handleRemove(labelValue: string) {
    const client = createServiceClient(SettingsService)
    await client.removeLabelPreset({ category: LABEL_CATEGORY, value: labelValue, workspaceId: '' })
    mutate()
  }

  async function handleColorChange(labelValue: string, newColor: string) {
    const client = createServiceClient(SettingsService)
    await client.updateLabelColor({
      category: LABEL_CATEGORY,
      value: labelValue,
      color: newColor,
      workspaceId: ''
    })
    mutate()
  }

  return (
    <section aria-labelledby="labels-heading">
      <h2
        id="labels-heading"
        className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted"
      >
        Label Presets
      </h2>
      {data.labelPresets.length === 0 ? (
        <p className="mb-3 text-sm text-muted">No label presets.</p>
      ) : (
        <div className="mb-3 space-y-2">
          {data.labelPresets.map((preset) => (
            <SettingsLabelRow
              key={preset.value}
              value={preset.value}
              color={preset.color}
              onColorChange={handleColorChange}
              onRemove={handleRemove}
            />
          ))}
        </div>
      )}
      {adding ? (
        <form onSubmit={handleAdd} className="flex items-center gap-2">
          <input
            type="color"
            value={color}
            onChange={(e) => setColor(e.target.value)}
            className="h-9 w-9 cursor-pointer rounded-full border-0 bg-transparent p-0"
          />
          <Input
            type="text"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="Label name"
            autoFocus
            className="flex-1"
          />
          <Button type="submit" size="sm" disabled={!value.trim()}>
            Add
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={() => setAdding(false)}>
            Cancel
          </Button>
        </form>
      ) : (
        <Button
          type="button"
          variant="link"
          size="sm"
          className="self-start"
          onClick={() => setAdding(true)}
        >
          + Add label
        </Button>
      )}
    </section>
  )
}
