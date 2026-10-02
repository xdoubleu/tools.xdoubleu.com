import { toFraction } from '@/lib/recipes/fractions'

describe('toFraction', () => {
  it.each([
    [0, '0'],
    [-1, '0'],
    [0.5, '½'],
    [1.5, '1½'],
    [2, '2'],
    [0.33, '⅓'],
    [0.66, '⅔'],
    [1.25, '1¼'],
    [0.125, '⅛'],
    [0.98, '1'],
    [0.01, '0'],
    [2.0625, '2⅛'],
    [0.375, '⅜'],
    [0.625, '⅝'],
    [0.75, '¾'],
    [1.875, '1⅞'],
    [5.97, '6']
  ])('%d → %s', (value, expected) => {
    expect(toFraction(value)).toBe(expected)
  })
})
