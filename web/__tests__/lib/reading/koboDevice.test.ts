import { defaultDeviceName } from '@/lib/reading/koboDevice'

describe('defaultDeviceName', () => {
  it('returns Kobo with last-4 suffix when serial has 4+ chars', () => {
    expect(defaultDeviceName('N418ABCD1234')).toBe('Kobo (…1234)')
  })

  it('uses exactly the last 4 chars', () => {
    expect(defaultDeviceName('WXYZ')).toBe('Kobo (…WXYZ)')
  })

  it('falls back to "My Kobo" when serial is too short', () => {
    expect(defaultDeviceName('ABC')).toBe('My Kobo')
    expect(defaultDeviceName('')).toBe('My Kobo')
  })
})
