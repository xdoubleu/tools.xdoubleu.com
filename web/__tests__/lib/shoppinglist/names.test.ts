import { hasName } from '@/lib/shoppinglist/names'

describe('hasName', () => {
  const list = [
    { id: '1', name: 'Produce' },
    { id: '2', name: 'Dairy' }
  ]

  it('matches names case-insensitively', () => {
    expect(hasName(list, 'produce')).toBe(true)
    expect(hasName(list, 'Bakery')).toBe(false)
  })

  it('ignores the entry being renamed', () => {
    expect(hasName(list, 'PRODUCE', '1')).toBe(false)
    expect(hasName(list, 'dairy', '1')).toBe(true)
  })
})
