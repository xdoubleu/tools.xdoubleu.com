'use client'

import { useState } from 'react'
import { useCreatePlan, useUpdatePlan } from '@/hooks/useMealPlans'
import { CreatePlanRequest, UpdatePlanRequest } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Plan } from '@/lib/gen/mealplans/v1/mealplans_pb'

interface PlanFormProps {
  plan?: Plan
  onSave: (id: string) => void
  onCancel: () => void
}

const SLOT_NAMES = ['breakfast', 'noon', 'evening']

export default function PlanForm({ plan, onSave, onCancel }: PlanFormProps) {
  const [name, setName] = useState(plan?.name ?? '')
  const [hiddenSlots, setHiddenSlots] = useState<string[]>(plan?.icalHideSlots ?? [])
  const [hidePast, setHidePast] = useState(plan?.icalHidePast ?? false)
  const [error, setError] = useState<string | null>(null)

  const createPlan = useCreatePlan()
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
      if (plan?.id) {
        await updatePlan(
          new UpdatePlanRequest({
            id: plan.id,
            name,
            icalHideSlots: hiddenSlots,
            icalHidePast: hidePast
          })
        )
        onSave(plan.id)
      } else {
        const result = await createPlan(new CreatePlanRequest({ name }))
        onSave(result.plan?.id ?? '')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save plan.')
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label htmlFor="plan-name" className="block text-sm font-medium text-subtle mb-1">
          Plan Name
        </label>
        <input
          id="plan-name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <p className="block text-sm font-medium text-subtle mb-2">iCal — Hide meal slots</p>
        <div className="flex flex-wrap gap-4">
          {SLOT_NAMES.map((slot) => (
            <label key={slot} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={hiddenSlots.includes(slot)}
                onChange={() => toggleSlot(slot)}
                className="h-4 w-4 rounded border-input-border"
              />
              {slot.charAt(0).toUpperCase() + slot.slice(1)}
            </label>
          ))}
        </div>
      </div>

      <div>
        <label htmlFor="ical-hide-past" className="flex items-center gap-2 text-sm">
          <input
            id="ical-hide-past"
            type="checkbox"
            checked={hidePast}
            onChange={(e) => setHidePast(e.target.checked)}
            className="h-4 w-4 rounded border-input-border"
          />
          <span className="font-medium text-subtle">iCal — Hide past events</span>
        </label>
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <div className="flex gap-2">
        <button
          type="submit"
          className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          Save Plan
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
