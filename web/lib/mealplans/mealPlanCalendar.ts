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
