import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  type ShoppingItem,
  type DayItems
} from '@/lib/recipes/shoppingExport'

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '0.5', unit: 'tsp', name: 'salt' }]

const dayItems: DayItems[] = [
  {
    date: '2026-05-26',
    items: [
      { amount: '2', unit: 'cups', name: 'flour' },
      { amount: '1', unit: 'tbsp', name: 'sugar' }
    ]
  },
  {
    date: '2026-05-27',
    items: [{ amount: '100', unit: 'g', name: 'butter' }]
  }
]

describe('shoppingExport', () => {
  describe('formatForClipboard', () => {
    it('formats custom items only when no day items given', () => {
      const result = formatForClipboard(customItems)
      expect(result).toBe('Custom items:\n0.5 tsp - salt')
    })

    it('formats custom items and per-day meal plan sections', () => {
      const result = formatForClipboard(customItems, dayItems)
      expect(result).toContain('Custom items:\n0.5 tsp - salt')
      expect(result).toContain('From meal plan:')
      expect(result).toContain('2026-05-26:')
      expect(result).toContain('  2 cups - flour')
      expect(result).toContain('2026-05-27:')
      expect(result).toContain('  100 g - butter')
    })

    it('omits meal plan section when day items array is empty', () => {
      const result = formatForClipboard(customItems, [])
      expect(result).toBe('Custom items:\n0.5 tsp - salt')
      expect(result).not.toContain('From meal plan:')
    })

    it('omits custom section when custom items are empty', () => {
      const result = formatForClipboard([], dayItems)
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

    it('includes custom items and per-day meal plan sections after title', () => {
      const result = formatForAppleNotes(customItems, dayItems, fixedDate)
      expect(result).toContain('Custom items:\n0.5 tsp salt')
      expect(result).toContain('From meal plan:')
      expect(result).toContain('2026-05-26:')
    })

    it('returns just the title when both are empty', () => {
      const result = formatForAppleNotes([], [], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })

    it('omits empty sections', () => {
      const result = formatForAppleNotes([], dayItems, fixedDate)
      expect(result).not.toContain('Custom items:')
      expect(result).toContain('From meal plan:')
    })
  })

  describe('formatAsTxt', () => {
    it('produces same output as clipboard format', () => {
      expect(formatAsTxt(customItems, dayItems)).toBe(formatForClipboard(customItems, dayItems))
    })

    it('produces same output without day items', () => {
      expect(formatAsTxt(customItems)).toBe(formatForClipboard(customItems))
    })
  })
})
