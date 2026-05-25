import { parseFraction } from '@/lib/recipes/parseFraction'

describe('parseFraction', () => {
  it('returns 0 for empty string', () => {
    expect(parseFraction('')).toBe(0)
    expect(parseFraction('  ')).toBe(0)
  })

  it('parses plain integers', () => {
    expect(parseFraction('1')).toBe(1)
    expect(parseFraction('3')).toBe(3)
  })

  it('parses plain decimals', () => {
    expect(parseFraction('0.5')).toBe(0.5)
    expect(parseFraction('1.25')).toBe(1.25)
  })

  it('parses simple fractions', () => {
    expect(parseFraction('1/2')).toBeCloseTo(0.5)
    expect(parseFraction('1/3')).toBeCloseTo(1 / 3)
    expect(parseFraction('2/3')).toBeCloseTo(2 / 3)
    expect(parseFraction('3/4')).toBeCloseTo(0.75)
  })

  it('parses mixed numbers', () => {
    expect(parseFraction('1 1/2')).toBeCloseTo(1.5)
    expect(parseFraction('2 1/4')).toBeCloseTo(2.25)
    expect(parseFraction('1 1/3')).toBeCloseTo(1 + 1 / 3)
  })

  it('parses unicode fraction symbols', () => {
    expect(parseFraction('½')).toBe(0.5)
    expect(parseFraction('¼')).toBe(0.25)
    expect(parseFraction('¾')).toBe(0.75)
    expect(parseFraction('⅓')).toBeCloseTo(1 / 3)
    expect(parseFraction('⅔')).toBeCloseTo(2 / 3)
    expect(parseFraction('⅛')).toBe(0.125)
    expect(parseFraction('⅜')).toBe(0.375)
    expect(parseFraction('⅝')).toBe(0.625)
    expect(parseFraction('⅞')).toBe(0.875)
  })

  it('parses whole number + unicode fraction', () => {
    expect(parseFraction('1½')).toBeCloseTo(1.5)
    expect(parseFraction('2¼')).toBeCloseTo(2.25)
    expect(parseFraction('3⅓')).toBeCloseTo(3 + 1 / 3)
  })

  it('returns 0 for invalid input', () => {
    expect(parseFraction('abc')).toBe(0)
    expect(parseFraction('one half')).toBe(0)
  })
})
