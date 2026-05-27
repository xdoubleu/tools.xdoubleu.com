import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  type ShoppingItem
} from '@/lib/recipes/shoppingExport'

const mealPlanItems: ShoppingItem[] = [
  { amount: '2', unit: 'cups', name: 'flour' },
  { amount: '1', unit: 'tbsp', name: 'sugar' }
]

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '0.5', unit: 'tsp', name: 'salt' }]

describe('shoppingExport', () => {
  describe('formatForClipboard', () => {
    it('formats both sections with labels', () => {
      const result = formatForClipboard(mealPlanItems, customItems)
      expect(result).toBe(
        'From meal plan:\n2 cups - flour\n1 tbsp - sugar\n\nCustom items:\n0.5 tsp - salt'
      )
    })

    it('omits meal plan section when empty', () => {
      const result = formatForClipboard([], customItems)
      expect(result).toBe('Custom items:\n0.5 tsp - salt')
    })

    it('omits custom section when empty', () => {
      const result = formatForClipboard(mealPlanItems, [])
      expect(result).toBe('From meal plan:\n2 cups - flour\n1 tbsp - sugar')
    })

    it('returns empty string when both arrays are empty', () => {
      expect(formatForClipboard([], [])).toBe('')
    })
  })

  describe('formatForAppleNotes', () => {
    const fixedDate = new Date(2026, 4, 26)

    it('includes date title', () => {
      const result = formatForAppleNotes(mealPlanItems, customItems, fixedDate)
      expect(result.startsWith('Shopping list 26/05/2026')).toBe(true)
    })

    it('includes both labeled sections after title', () => {
      const result = formatForAppleNotes(mealPlanItems, customItems, fixedDate)
      expect(result).toContain('From meal plan:\n2 cups flour\n1 tbsp sugar')
      expect(result).toContain('Custom items:\n0.5 tsp salt')
    })

    it('returns just the title when both arrays are empty', () => {
      const result = formatForAppleNotes([], [], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })

    it('omits empty sections', () => {
      const result = formatForAppleNotes(mealPlanItems, [], fixedDate)
      expect(result).not.toContain('Custom items:')
      expect(result).toContain('From meal plan:')
    })
  })

  describe('formatAsTxt', () => {
    it('produces same output as clipboard format', () => {
      const result = formatAsTxt(mealPlanItems, customItems)
      expect(result).toBe(formatForClipboard(mealPlanItems, customItems))
    })
  })
})
