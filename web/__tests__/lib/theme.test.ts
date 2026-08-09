import { getTheme, setTheme, themeInitScript, THEME_KEY } from '@/lib/theme'

/** Stubs matchMedia with a single mutable media-query-list, returned for inspection. */
function mockPrefersDark(matches: boolean) {
  const mql = {
    matches,
    listeners: [] as (() => void)[],
    addEventListener(_: string, fn: () => void) {
      mql.listeners.push(fn)
    }
  }
  Object.defineProperty(window, 'matchMedia', { value: () => mql, writable: true })
  return mql
}

describe('theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
    document.documentElement.style.colorScheme = ''
  })

  describe('getTheme', () => {
    it('defaults to auto when nothing is stored', () => {
      expect(getTheme()).toBe('auto')
    })

    it('returns the stored theme', () => {
      localStorage.setItem(THEME_KEY, 'dark')
      expect(getTheme()).toBe('dark')
    })

    it('falls back to auto for an unknown stored value', () => {
      localStorage.setItem(THEME_KEY, 'sepia')
      expect(getTheme()).toBe('auto')
    })
  })

  describe('setTheme', () => {
    it.each([
      ['dark', false, 'dark'],
      ['light', true, 'light'],
      ['auto', true, 'dark'],
      ['auto', false, 'light']
    ] as const)('%s with system dark=%s resolves to %s', (theme, systemDark, resolved) => {
      mockPrefersDark(systemDark)
      setTheme(theme)
      expect(localStorage.getItem(THEME_KEY)).toBe(theme)
      expect(document.documentElement.dataset.theme).toBe(resolved)
      expect(document.documentElement.style.colorScheme).toBe(resolved)
    })

    it('falls back to auto for an unrecognised value', () => {
      mockPrefersDark(true)
      expect(setTheme('sepia')).toBe('auto')
      expect(localStorage.getItem(THEME_KEY)).toBe('auto')
      expect(document.documentElement.dataset.theme).toBe('dark')
    })
  })

  describe('themeInitScript', () => {
    it.each([
      ['dark', false, 'dark'],
      ['light', true, 'light'],
      [null, true, 'dark'],
      [null, false, 'light']
    ])('stored=%s system dark=%s resolves to %s', (stored, systemDark, resolved) => {
      mockPrefersDark(systemDark as boolean)
      if (stored) localStorage.setItem(THEME_KEY, stored as string)
      eval(themeInitScript)
      expect(document.documentElement.dataset.theme).toBe(resolved)
      expect(document.documentElement.style.colorScheme).toBe(resolved)
    })

    it('re-resolves on a system scheme change while on auto', () => {
      const mql = mockPrefersDark(false)
      eval(themeInitScript)
      expect(document.documentElement.dataset.theme).toBe('light')

      mql.matches = true
      mql.listeners.forEach((fn) => fn())
      expect(document.documentElement.dataset.theme).toBe('dark')
    })

    it('ignores a system scheme change when a theme is forced', () => {
      const mql = mockPrefersDark(false)
      localStorage.setItem(THEME_KEY, 'light')
      eval(themeInitScript)

      mql.matches = true
      mql.listeners.forEach((fn) => fn())
      expect(document.documentElement.dataset.theme).toBe('light')
    })
  })
})
