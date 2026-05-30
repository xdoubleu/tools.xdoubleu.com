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
  describe('unit upgrades', () => {
    it('converts 1000g to 1kg', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'g', name: 'flour' }])
      expect(result).toBe('1 kg - flour')
    })

    it('converts 1500g to 1.5kg', () => {
      const result = formatForClipboard([{ amount: '1500', unit: 'g', name: 'flour' }])
      expect(result).toBe('1.5 kg - flour')
    })

    it('does not convert 999g', () => {
      const result = formatForClipboard([{ amount: '999', unit: 'g', name: 'flour' }])
      expect(result).toBe('999 g - flour')
    })

    it('converts 1000ml to 1L', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'ml', name: 'water' }])
      expect(result).toBe('1 L - water')
    })

    it('converts 1000mg to 1g', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'mg', name: 'spice' }])
      expect(result).toBe('1 g - spice')
    })

    it('does not convert unknown units', () => {
      const result = formatForClipboard([{ amount: '1000', unit: 'tsp', name: 'salt' }])
      expect(result).toBe('1000 tsp - salt')
    })
  })

  describe('formatForClipboard', () => {
    it('formats custom items only when no meal items given', () => {
      const result = formatForClipboard(customItems)
      expect(result).toBe('0.5 tsp - salt')
    })

    it('merges custom and meal plan items into a single flat list', () => {
      const result = formatForClipboard(customItems, mealItems)
      expect(result).toBe('0.5 tsp - salt\n2 cups - flour\n1 tbsp - sugar\n100 g - butter')
    })

    it('omits meal plan items when meal items array is empty', () => {
      const result = formatForClipboard(customItems, [])
      expect(result).toBe('0.5 tsp - salt')
    })

    it('formats meal items only when custom items are empty', () => {
      const result = formatForClipboard([], mealItems)
      expect(result).toBe('2 cups - flour\n1 tbsp - sugar\n100 g - butter')
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

    it('merges custom and meal plan items into a single flat list after title', () => {
      const result = formatForAppleNotes(customItems, mealItems, fixedDate)
      expect(result).toBe(
        'Shopping list 26/05/2026\n\n0.5 tsp salt\n2 cups flour\n1 tbsp sugar\n100 g butter'
      )
    })

    it('returns just the title when both are empty', () => {
      const result = formatForAppleNotes([], [], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })

    it('formats meal items only when custom items are empty', () => {
      const result = formatForAppleNotes([], mealItems, fixedDate)
      expect(result).toBe('Shopping list 26/05/2026\n\n2 cups flour\n1 tbsp sugar\n100 g butter')
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
