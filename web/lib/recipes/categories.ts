/** Recipe category values, matching the api's allowed meal types and effort levels. */
export const MEAL_TYPES = [
  { value: 'light_meal', label: 'Light meal' },
  { value: 'main_meal', label: 'Main meal' }
] as const

export const EFFORTS = [
  { value: 'low', label: 'Low effort' },
  { value: 'high', label: 'High effort' }
] as const

function labelFor(options: readonly { value: string; label: string }[], value: string) {
  return options.find((o) => o.value === value)?.label
}

export const mealTypeLabel = (value: string) => labelFor(MEAL_TYPES, value)
export const effortLabel = (value: string) => labelFor(EFFORTS, value)
