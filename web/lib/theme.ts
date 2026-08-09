/**
 * Theme preference, stored per device in localStorage and applied as
 * `data-theme` on <html> (see `app/globals.css`). No React imports — this
 * module is read from both the server layout and client components.
 */

export const THEMES = ['auto', 'light', 'dark'] as const

export type Theme = (typeof THEMES)[number]

export const THEME_KEY = 'theme'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function isTheme(value: string | null): value is Theme {
  return THEMES.some((theme) => theme === value)
}

export function getTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY)
  return isTheme(stored) ? stored : 'auto'
}

/** Stores and applies a theme, ignoring anything unrecognised; returns what was applied. */
export function setTheme(value: string): Theme {
  const theme = isTheme(value) ? value : 'auto'
  localStorage.setItem(THEME_KEY, theme)
  const dark = theme === 'dark' || (theme === 'auto' && window.matchMedia(DARK_QUERY).matches)
  const el = document.documentElement
  el.dataset.theme = dark ? 'dark' : 'light'
  el.style.colorScheme = dark ? 'dark' : 'light'
  return theme
}

/*
 * ponytail: the same resolve-and-apply logic, hand-inlined for <head> — it must
 * run before first paint (no flash) and before any bundle loads, so it can't
 * import from here. Also re-runs on OS scheme changes to keep `auto` live.
 */
export const themeInitScript = `(function(){var m=matchMedia('${DARK_QUERY}');function a(){var t=localStorage.getItem('${THEME_KEY}');var d=t==='dark'||(t!=='light'&&m.matches);var e=document.documentElement;e.dataset.theme=d?'dark':'light';e.style.colorScheme=d?'dark':'light'}a();m.addEventListener('change',a)})()`
