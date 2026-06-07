import {
  parseCustomItems,
  encodeCustomItems,
  formatCustomItemLabel,
  formatCustomNameLabel
} from '@/lib/customItems'

describe('customItems', () => {
  it('parses bare names and name/amount pairs', () => {
    expect(parseCustomItems('Rice\nOlive oil\t2')).toEqual([
      { name: 'Rice', amount: '' },
      { name: 'Olive oil', amount: '2' }
    ])
  })

  it('drops blank lines when parsing', () => {
    expect(parseCustomItems('\nRice\n')).toEqual([{ name: 'Rice', amount: '' }])
  })

  it('encodes amounts with a tab and omits empty amounts', () => {
    expect(
      encodeCustomItems([
        { name: 'Rice', amount: '' },
        { name: 'Olive oil', amount: '2' }
      ])
    ).toBe('Rice\nOlive oil\t2')
  })

  it('drops items with blank names and trims when encoding', () => {
    expect(
      encodeCustomItems([
        { name: '  ', amount: '5' },
        { name: ' Eggs ', amount: ' 3 ' }
      ])
    ).toBe('Eggs\t3')
  })

  it('round-trips through encode and parse', () => {
    const items = [
      { name: 'Rice', amount: '' },
      { name: 'Olive oil', amount: '2' }
    ]
    expect(parseCustomItems(encodeCustomItems(items))).toEqual(items)
  })

  it('formats a label with and without an amount', () => {
    expect(formatCustomItemLabel({ name: 'Apples', amount: '3' })).toBe('3 Apples')
    expect(formatCustomItemLabel({ name: 'Apples', amount: '' })).toBe('Apples')
  })

  it('formats a whole customName for display', () => {
    expect(formatCustomNameLabel('Rice\nOlive oil\t2')).toBe('Rice\n2 Olive oil')
  })

  it('ignores the UI-only categoryId when encoding', () => {
    expect(
      encodeCustomItems([
        { name: 'Rice', amount: '', categoryId: 'cat-1' },
        { name: 'Olive oil', amount: '2', categoryId: 'cat-2' }
      ])
    ).toBe('Rice\nOlive oil\t2')
  })
})
