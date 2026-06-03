export const MEAL_SLOTS = ['breakfast', 'noon', 'evening'] as const

export function getWeekDates(offsetWeeks: number): Date[] {
  const today = new Date()
  const dayOfWeek = today.getDay() // 0=Sun, 1=Mon...6=Sat
  const daysFromMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1
  const monday = new Date(today)
  monday.setDate(today.getDate() - daysFromMonday + offsetWeeks * 7)

  const dates: Date[] = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(monday)
    d.setDate(monday.getDate() + i)
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
