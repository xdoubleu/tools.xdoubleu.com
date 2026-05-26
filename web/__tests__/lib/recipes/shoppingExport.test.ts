import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  type ShoppingItem
} from '@/lib/recipes/shoppingExport'

const mockItems: ShoppingItem[] = [
  { amount: '2', unit: 'cups', name: 'flour' },
  { amount: '1', unit: 'tbsp', name: 'sugar' },
  { amount: '0.5', unit: 'tsp', name: 'salt' }
]

describe('shoppingExport', () => {
  describe('formatForClipboard', () => {
    it('should format items as "amount unit - name"', () => {
      const result = formatForClipboard(mockItems)
      const lines = result.split('\n')
      expect(lines[0]).toBe('2 cups - flour')
      expect(lines[1]).toBe('1 tbsp - sugar')
      expect(lines[2]).toBe('0.5 tsp - salt')
    })

    it('should handle empty array', () => {
      const result = formatForClipboard([])
      expect(result).toBe('')
    })
  })

  describe('formatForAppleNotes', () => {
    const fixedDate = new Date(2026, 4, 26)

    it('should include a title with the date', () => {
      const result = formatForAppleNotes(mockItems, fixedDate)
      const lines = result.split('\n')
      expect(lines[0]).toBe('Shopping list 26/05/2026')
    })

    it('should format items one per line without prefix', () => {
      const result = formatForAppleNotes(mockItems, fixedDate)
      const lines = result.split('\n')
      expect(lines[1]).toBe('2 cups flour')
      expect(lines[2]).toBe('1 tbsp sugar')
      expect(lines[3]).toBe('0.5 tsp salt')
    })

    it('should return just the title for an empty array', () => {
      const result = formatForAppleNotes([], fixedDate)
      expect(result).toBe('Shopping list 26/05/2026')
    })
  })

  describe('formatAsTxt', () => {
    it('should format items same as clipboard format', () => {
      const result = formatAsTxt(mockItems)
      const clipboardResult = formatForClipboard(mockItems)
      expect(result).toBe(clipboardResult)
    })
  })
})
