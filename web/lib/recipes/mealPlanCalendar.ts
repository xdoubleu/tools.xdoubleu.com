export const MEAL_SLOTS = ['Breakfast', 'Noon', 'Evening'] as const

export function getWeekDates(offsetWeeks: number): Date[] {
  const today = new Date()
  const weekStart = new Date(today)

  // Get Monday of the current/offset week
  const day = weekStart.getDay()
  const diff = weekStart.getDate() - day + (day === 0 ? -6 : 1)
  weekStart.setDate(diff)

  // Apply week offset
  weekStart.setDate(weekStart.getDate() + offsetWeeks * 7)

  const dates: Date[] = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(weekStart)
    d.setDate(d.getDate() + i)
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
