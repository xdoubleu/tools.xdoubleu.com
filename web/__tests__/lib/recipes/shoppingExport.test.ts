import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  type ShoppingItem
} from '@/lib/recipes/shoppingExport'

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '0.5', unit: 'tsp', name: 'salt' }]

const mealItems: ShoppingItem[] = [
  { amount: '2', unit: 'cups', name: 'flour' },
  { amount: '1', unit: 'tbsp', name: 'sugar' },
  { amount: '100', unit: 'g', name: 'butter' }
]

describe('shoppingExport', () => {
  describe('formatForClipboard', () => {
    it('formats custom items only when no meal items given', () => {
      const result = formatForClipboard(customItems)
      expect(result).toBe('Custom items:\n0.5 tsp - salt')
    })

    it('formats custom items and aggregated meal plan section', () => {
      const result = formatForClipboard(customItems, mealItems)
      expect(result).toContain('Custom items:\n0.5 tsp - salt')
      expect(result).toContain('From meal plan:')
      expect(result).toContain('  2 cups - flour')
      expect(result).toContain('  100 g - butter')
    })

    it('omits meal plan section when meal items array is empty', () => {
      const result = formatForClipboard(customItems, [])
      expect(result).toBe('Custom items:\n0.5 tsp - salt')
      expect(result).not.toContain('From meal plan:')
    })

    it('omits custom section when custom items are empty', () => {
      const result = formatForClipboard([], mealItems)
      expect(result).not.toContain('Custom items:')
      expect(result).toContain('From meal plan:')
    })

    it('returns empty string when both are empty', () => {
      expect(formatForClipboard([], [])).toBe('')
    })
  })

  describe('formatForAppleNotes', () => {
    const fixedDate = new Date(2026, 4, 26)

    it('includes date title', () => {
      const result = formatForAppleNotes(customItems, undefined, fixedDate)
      expect(result.startsWith('Shopping list 26/05/2026')).toBe(true)
    })

    it('includes custom items and aggregated meal plan section after title', () => {
      const result = formatForAppleNotes(customItems, mealItems, fixedDate)
      expect(result).toContain('Custom items:\n0.5 tsp salt')
      expect(result).toContain('From meal plan:')
      expect(result).toContain('  2 cups flour')
    })

    it('returns just the title when both are empty', () => {
      const result = formatForAppleNotes([], [], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })

    it('omits empty sections', () => {
      const result = formatForAppleNotes([], mealItems, fixedDate)
      expect(result).not.toContain('Custom items:')
      expect(result).toContain('From meal plan:')
    })
  })

  describe('formatAsTxt', () => {
    it('produces same output as clipboard format', () => {
      expect(formatAsTxt(customItems, mealItems)).toBe(formatForClipboard(customItems, mealItems))
    })

    it('produces same output without meal items', () => {
      expect(formatAsTxt(customItems)).toBe(formatForClipboard(customItems))
    })
  })
})
