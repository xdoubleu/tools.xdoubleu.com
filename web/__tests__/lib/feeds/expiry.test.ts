import { EXPIRY_OPTIONS, expiresLabel, expiryOptionLabel } from '@/lib/feeds/expiry'

const NOW = Date.parse('2026-11-02T12:00:00Z')
const at = (ms: number) => new Date(NOW + ms).toISOString()
const HOUR = 3_600_000

describe('expiresLabel', () => {
  it('is null when the item never expires or has plenty of time', () => {
    expect(expiresLabel('', NOW)).toBeNull()
    expect(expiresLabel(at(49 * HOUR), NOW)).toBeNull()
    expect(expiresLabel('not a date', NOW)).toBeNull()
  })

  it('counts down in hours, then days', () => {
    expect(expiresLabel(at(30 * 60_000), NOW)).toBe('Expires within the hour')
    expect(expiresLabel(at(5.5 * HOUR), NOW)).toBe('Expires in 5h')
    expect(expiresLabel(at(30 * HOUR), NOW)).toBe('Expires tomorrow')
  })
})

describe('expiryOptionLabel', () => {
  it('names each lifetime, with 0 as never', () => {
    expect(EXPIRY_OPTIONS.map(expiryOptionLabel)).toEqual([
      '1 day',
      '3 days',
      '7 days',
      '14 days',
      '30 days',
      'Never'
    ])
  })
})
