import { formatNumber, formatPercent, formatQuantity, pluralize } from '@/lib/format'

describe('formatNumber', () => {
  it.each([
    [933.333, '933.3'],
    [12, '12'],
    [2.96, '3'],
    [2.345, '2.3'],
    [1234567.89, '1,234,567.9'],
    [-0.04, '0'],
    [-1.5, '-1.5'],
    ['66.50', '66.5'],
    [42n, '42']
  ])('%s is %s', (value, expected) => {
    expect(formatNumber(value)).toBe(expected)
  })

  it('takes a decimals limit', () => {
    expect(formatNumber(2.345, 2)).toBe('2.35')
    expect(formatNumber(2.345, 0)).toBe('2')
  })

  it('returns an empty string for a non-number', () => {
    expect(formatNumber('abc')).toBe('')
    expect(formatNumber('')).toBe('')
    expect(formatNumber('  ')).toBe('')
  })
})

describe('formatPercent', () => {
  it('trims to one decimal', () => {
    expect(formatPercent('66.50')).toBe('66.5%')
    expect(formatPercent(64.29)).toBe('64.3%')
    expect(formatPercent('')).toBe('')
    expect(formatPercent('x')).toBe('')
  })
})

describe('pluralize', () => {
  it('picks the form by count', () => {
    expect(pluralize(1, 'comment')).toBe('1 comment')
    expect(pluralize(0, 'comment')).toBe('0 comments')
    expect(pluralize(2, 'person', 'people')).toBe('2 people')
  })
})

describe('formatQuantity', () => {
  it('prints amount, unit and name', () => {
    expect(formatQuantity('500', 'g', 'coffee beans')).toBe('500 g coffee beans')
    expect(formatQuantity('933.333', 'g', 'chopped tomatoes')).toBe('933.33 g chopped tomatoes')
    expect(formatQuantity('0.25', 'kg', 'flour')).toBe('0.25 kg flour')
    expect(formatQuantity(6, '', 'bananas')).toBe('6 bananas')
    expect(formatQuantity(5, ' g ', 'rice')).toBe('5 g rice')
  })

  it('leaves just the name for a zero or missing amount', () => {
    expect(formatQuantity('0', '', "dinner at sam's")).toBe("dinner at sam's")
    expect(formatQuantity(undefined, 'g', 'salt')).toBe('salt')
    expect(formatQuantity(0, 'g', 'salt')).toBe('salt')
    expect(formatQuantity('abc', 'g', 'salt')).toBe('salt')
  })
})
