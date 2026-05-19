import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'

describe('mealPlanCalendar', () => {
  describe('MEAL_SLOTS', () => {
    it('should contain the three slots', () => {
      expect(MEAL_SLOTS).toEqual(['Breakfast', 'Noon', 'Evening'])
    })
  })

  describe('getWeekDates', () => {
    it('should return 7 dates', () => {
      const dates = getWeekDates(0)
      expect(dates).toHaveLength(7)
    })

    it('should return dates for the current week (offset 0)', () => {
      const dates = getWeekDates(0)
      const startDate = dates[0]

      // Should be a Monday (day 1 in JS getDay)
      expect(startDate.getDay()).toBe(1)

      // Each subsequent date should be 1 day later
      for (let i = 1; i < 7; i++) {
        const diff = (dates[i].getTime() - dates[i - 1].getTime()) / (1000 * 60 * 60 * 24)
        expect(diff).toBe(1)
      }
    })

    it('should return dates for next week (offset 1)', () => {
      const thisWeek = getWeekDates(0)
      const nextWeek = getWeekDates(1)

      // nextWeek[0] should be 7 days after thisWeek[0]
      const dayDiff = (nextWeek[0].getTime() - thisWeek[0].getTime()) / (1000 * 60 * 60 * 24)
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
