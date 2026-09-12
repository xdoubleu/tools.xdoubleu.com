'use client'

import { useState } from 'react'
import { useCreateLearningPath, useUpdateLearningPath } from '@/hooks/useLearningPaths'
import type { CreateLearningPathInput, UpdateLearningPathInput } from '@/hooks/useLearningPaths'
import type { LearningPath } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface LearningPathFormProps {
  learningPath?: LearningPath
  onSave: (id: string) => void
  onCancel: () => void
}

interface ItemRow {
  type: string
  description: string
  completed: boolean
}

interface ModuleRow {
  title: string
  items: ItemRow[]
}

const emptyItem = (): ItemRow => ({ type: '', description: '', completed: false })
const emptyModule = (): ModuleRow => ({ title: '', items: [emptyItem()] })

export default function LearningPathForm({
  learningPath,
  onSave,
  onCancel
}: LearningPathFormProps) {
  const [title, setTitle] = useState(learningPath?.title || '')
  const [goal, setGoal] = useState(learningPath?.goal || '')
  const [routine, setRoutine] = useState(learningPath?.routine || '')
  const [modules, setModules] = useState<ModuleRow[]>(
    learningPath?.modules?.length
      ? learningPath.modules.map((m) => ({
          title: m.title,
          items: m.items.length
            ? m.items.map((i) => ({
                type: i.type,
                description: i.description,
                completed: i.completed
              }))
            : [emptyItem()]
        }))
      : [emptyModule()]
  )
  const [resources, setResources] = useState<string[]>(
    learningPath?.resources?.length ? learningPath.resources.map((r) => r.text) : ['']
  )

  const createLearningPath = useCreateLearningPath()
  const updateLearningPath = useUpdateLearningPath()

  const addModule = () => setModules([...modules, emptyModule()])
  const removeModule = (idx: number) => setModules(modules.filter((_, i) => i !== idx))
  const updateModuleTitle = (idx: number, value: string) => {
    const updated = [...modules]
    updated[idx] = { ...updated[idx], title: value }
    setModules(updated)
  }

  const addItem = (moduleIdx: number) => {
    const updated = [...modules]
    updated[moduleIdx] = {
      ...updated[moduleIdx],
      items: [...updated[moduleIdx].items, emptyItem()]
    }
    setModules(updated)
  }
  const removeItem = (moduleIdx: number, itemIdx: number) => {
    const updated = [...modules]
    updated[moduleIdx] = {
      ...updated[moduleIdx],
      items: updated[moduleIdx].items.filter((_, i) => i !== itemIdx)
    }
    setModules(updated)
  }
  const updateItem = (
    moduleIdx: number,
    itemIdx: number,
    field: 'type' | 'description',
    value: string
  ) => {
    const updated = [...modules]
    const items = [...updated[moduleIdx].items]
    items[itemIdx] = { ...items[itemIdx], [field]: value }
    updated[moduleIdx] = { ...updated[moduleIdx], items }
    setModules(updated)
  }

  const addResource = () => setResources([...resources, ''])
  const removeResource = (idx: number) => setResources(resources.filter((_, i) => i !== idx))
  const updateResource = (idx: number, value: string) => {
    const updated = [...resources]
    updated[idx] = value
    setResources(updated)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const modulePayload = modules
        .filter((m) => m.title.trim())
        .map((m) => ({
          title: m.title,
          items: m.items
            .filter((i) => i.description.trim())
            .map((i) => ({ type: i.type, description: i.description, completed: i.completed }))
        }))
      const resourcePayload = resources.filter((r) => r.trim()).map((text) => ({ text }))

      const base = {
        title,
        goal,
        routine,
        modules: modulePayload,
        resources: resourcePayload
      }

      let savedId: string
      if (learningPath?.id) {
        const req: UpdateLearningPathInput = { id: learningPath.id, ...base }
        const result = await updateLearningPath(req)
        savedId = result.learningPath?.id || learningPath.id
      } else {
        const req: CreateLearningPathInput = base
        const result = await createLearningPath(req)
        savedId = result.learningPath?.id || ''
      }

      onSave(savedId)
    } catch (err) {
      console.error('Failed to save learning path:', err)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-1.5">
        <Label>Title</Label>
        <Input type="text" value={title} onChange={(e) => setTitle(e.target.value)} required />
      </div>

      <div className="space-y-1.5">
        <Label>Goal</Label>
        <Textarea value={goal} onChange={(e) => setGoal(e.target.value)} rows={2} />
      </div>

      <div className="space-y-1.5">
        <Label>Recurring routine</Label>
        <p className="text-xs text-muted-foreground">
          e.g. &ldquo;30 min every weekday morning&rdquo;
        </p>
        <Input type="text" value={routine} onChange={(e) => setRoutine(e.target.value)} />
      </div>

      <div className="space-y-1.5">
        <Label>Modules</Label>
        <div className="space-y-3">
          {modules.map((module, mIdx) => (
            <div key={mIdx} className="rounded-2xl border border-border bg-surface/50 p-3">
              <div className="flex gap-2 mb-2">
                <Input
                  type="text"
                  placeholder="Module title (e.g. Month 1)"
                  value={module.title}
                  onChange={(e) => updateModuleTitle(mIdx, e.target.value)}
                  className="flex-1"
                />
                {modules.length > 1 && (
                  <Button
                    type="button"
                    variant="destructive"
                    size="sm"
                    aria-label="Remove module"
                    onClick={() => removeModule(mIdx)}
                  >
                    ×
                  </Button>
                )}
              </div>
              <div className="space-y-2 pl-2">
                {module.items.map((item, iIdx) => (
                  <div key={iIdx} className="flex flex-wrap gap-2">
                    <Input
                      type="text"
                      placeholder="Type (read/study/do/checkpoint)"
                      value={item.type}
                      onChange={(e) => updateItem(mIdx, iIdx, 'type', e.target.value)}
                      className="w-48"
                    />
                    <Input
                      type="text"
                      placeholder="Description"
                      value={item.description}
                      onChange={(e) => updateItem(mIdx, iIdx, 'description', e.target.value)}
                      className="flex-1 min-w-40"
                    />
                    {module.items.length > 1 && (
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        aria-label="Remove item"
                        onClick={() => removeItem(mIdx, iIdx)}
                      >
                        ×
                      </Button>
                    )}
                  </div>
                ))}
                <Button type="button" variant="secondary" size="sm" onClick={() => addItem(mIdx)}>
                  Add Item
                </Button>
              </div>
            </div>
          ))}
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={addModule}>
          Add Module
        </Button>
      </div>

      <div className="space-y-1.5">
        <Label>Resources</Label>
        <p className="text-xs text-muted-foreground">Freeform for now — books, sites, tools.</p>
        <div className="space-y-2">
          {resources.map((resource, idx) => (
            <div key={idx} className="flex gap-2">
              <Input
                type="text"
                placeholder="e.g. Book: The Go Programming Language"
                value={resource}
                onChange={(e) => updateResource(idx, e.target.value)}
                className="flex-1"
              />
              {resources.length > 1 && (
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  aria-label="Remove resource"
                  onClick={() => removeResource(idx)}
                >
                  ×
                </Button>
              )}
            </div>
          ))}
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={addResource}>
          Add Resource
        </Button>
      </div>

      <div className="flex gap-2">
        <Button type="submit" className="flex-1">
          Save Learning Path
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
          Cancel
        </Button>
      </div>
    </form>
  )
}
