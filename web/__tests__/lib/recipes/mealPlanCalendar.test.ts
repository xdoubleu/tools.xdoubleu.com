import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'

describe('mealPlanCalendar', () => {
  describe('MEAL_SLOTS', () => {
    it('should contain the three slots', () => {
      expect(MEAL_SLOTS).toEqual(['breakfast', 'noon', 'evening'])
    })
  })

  describe('getWeekDates', () => {
    it('should return 7 dates', () => {
      const dates = getWeekDates(0)
      expect(dates).toHaveLength(7)
    })

    it('should start from Monday of the current week at offset 0', () => {
      jest.useFakeTimers()
      // June 3 2026 is a Wednesday — Monday of that week is June 1
      // Use local-time constructor to avoid UTC-offset shifting the date
      jest.setSystemTime(new Date(2026, 5, 3, 12, 0, 0))
      try {
        const dates = getWeekDates(0)
        expect(formatMealDate(dates[0])).toBe('2026-06-01')
        // Each subsequent date should be 1 day later (Math.round handles DST hour shifts)
        for (let i = 1; i < 7; i++) {
          const diff = Math.round(
            (dates[i].getTime() - dates[i - 1].getTime()) / (1000 * 60 * 60 * 24)
          )
          expect(diff).toBe(1)
        }
      } finally {
        jest.useRealTimers()
      }
    })

    it('should start from Monday when today is Sunday', () => {
      jest.useFakeTimers()
      // June 7 2026 is a Sunday — Monday of that week is June 1
      jest.setSystemTime(new Date(2026, 5, 7, 12, 0, 0))
      try {
        const dates = getWeekDates(0)
        expect(formatMealDate(dates[0])).toBe('2026-06-01')
      } finally {
        jest.useRealTimers()
      }
    })

    it('should start from Monday when today is Monday', () => {
      jest.useFakeTimers()
      // June 1 2026 is a Monday
      jest.setSystemTime(new Date(2026, 5, 1, 12, 0, 0))
      try {
        const dates = getWeekDates(0)
        expect(formatMealDate(dates[0])).toBe('2026-06-01')
      } finally {
        jest.useRealTimers()
      }
    })

    it('should return dates for next week (offset 1)', () => {
      const thisWeek = getWeekDates(0)
      const nextWeek = getWeekDates(1)

      // nextWeek[0] should be 7 days after thisWeek[0]
      const dayDiff = Math.round(
        (nextWeek[0].getTime() - thisWeek[0].getTime()) / (1000 * 60 * 60 * 24)
      )
      expect(dayDiff).toBe(7)
    })
  })

  describe('formatMealDate', () => {
    it('should format date as YYYY-MM-DD', () => {
      const d = new Date('2024-01-15')
      const formatted = formatMealDate(d)
      expect(formatted).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    })

    it('should pad month and day with zeros', () => {
      const d = new Date('2024-01-05')
      const formatted = formatMealDate(d)
      expect(formatted).toBe('2024-01-05')
    })
  })
})
