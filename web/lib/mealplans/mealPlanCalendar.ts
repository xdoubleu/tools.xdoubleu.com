export const MEAL_SLOTS = ['breakfast', 'noon', 'evening'] as const

export function getWeekDates(offsetWeeks: number): Date[] {
  // Rolling 7-day window anchored on today (today is always the first day).
  const start = new Date()
  start.setDate(start.getDate() + offsetWeeks * 7)

  const dates: Date[] = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(start)
    d.setDate(start.getDate() + i)
    dates.push(d)
  }

  return dates
}

export function formatMealDate(d: Date): string {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/** Orders meals as the server does: creation time, then id. */
export function compareMeals(
  a: { createdAtMicros: bigint; id: string },
  b: { createdAtMicros: bigint; id: string }
): number {
  if (a.createdAtMicros !== b.createdAtMicros) return a.createdAtMicros < b.createdAtMicros ? -1 : 1
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
}
