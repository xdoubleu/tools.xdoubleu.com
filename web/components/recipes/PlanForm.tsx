'use client'

import { useState } from 'react'
import { useUpdatePlan } from '@/hooks/useMealPlans'
import type { UpdatePlanInput } from '@/hooks/useMealPlans'
import type { Plan } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface PlanFormProps {
  plan: Plan
  onSave: (id: string) => void
  onCancel: () => void
}

const SLOT_NAMES = ['breakfast', 'noon', 'evening']

export default function PlanForm({ plan, onSave, onCancel }: PlanFormProps) {
  const [name, setName] = useState(plan.name)
  const [hiddenSlots, setHiddenSlots] = useState<string[]>(plan.icalHideSlots)
  const [hidePast, setHidePast] = useState(plan.icalHidePast)
  const [error, setError] = useState<string | null>(null)

  const updatePlan = useUpdatePlan()

  const toggleSlot = (slot: string) => {
    setHiddenSlots((prev) =>
      prev.includes(slot) ? prev.filter((s) => s !== slot) : [...prev, slot]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const req: UpdatePlanInput = {
        id: plan.id,
        name,
        icalHideSlots: hiddenSlots,
        icalHidePast: hidePast
      }
      await updatePlan(req)
      onSave(plan.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save plan.')
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="plan-name">Plan Name</Label>
        <Input
          id="plan-name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      </div>

      <div className="space-y-2">
        <p className="text-sm font-medium text-subtle">iCal — Hide meal slots</p>
        <div className="flex flex-wrap gap-4">
          {SLOT_NAMES.map((slot) => (
            <label key={slot} className="flex items-center gap-2 text-sm text-fg cursor-pointer">
              <input
                type="checkbox"
                checked={hiddenSlots.includes(slot)}
                onChange={() => toggleSlot(slot)}
                className="h-4 w-4 rounded accent-accent"
              />
              {slot.charAt(0).toUpperCase() + slot.slice(1)}
            </label>
          ))}
        </div>
      </div>

      <label
        htmlFor="ical-hide-past"
        className="flex items-center gap-2 text-sm text-fg cursor-pointer"
      >
        <input
          id="ical-hide-past"
          type="checkbox"
          checked={hidePast}
          onChange={(e) => setHidePast(e.target.checked)}
          className="h-4 w-4 rounded accent-accent"
        />
        <span className="font-medium">iCal — Hide past events</span>
      </label>

      {error && <p className="text-sm text-danger">{error}</p>}

      <div className="flex gap-2">
        <Button type="submit" className="flex-1">
          Save
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
          Cancel
        </Button>
      </div>
    </form>
  )
}
