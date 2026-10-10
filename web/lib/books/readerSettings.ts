// Pure helpers for the foliate-js book reader (components/books/reader/).

export type ReaderFormat = 'epub' | 'pdf'
export type ReaderTheme = 'light' | 'sepia' | 'dark'
export type PageTurn = 'left' | 'right'

/** Original formats the reader opens, in default-choice order. */
const READER_FORMATS: ReaderFormat[] = ['epub', 'pdf']

export const READER_THEMES: { value: ReaderTheme; label: string }[] = [
  { value: 'light', label: 'Light' },
  { value: 'sepia', label: 'Sepia' },
  { value: 'dark', label: 'Dark' }
]

// The book's own colours, independent of the app theme.
const THEME_COLORS: Record<ReaderTheme, { bg: string; fg: string; link: string }> = {
  light: { bg: '#ffffff', fg: '#1f1f1f', link: '#1d4ed8' },
  sepia: { bg: '#f4ecd8', fg: '#5b4636', link: '#8a4b14' },
  dark: { bg: '#161616', fg: '#e3e3e3', link: '#8ab4f8' }
}

export const READER_FONT_SIZE_DEFAULT = 100
export const READER_FONT_SIZE_MIN = 80
export const READER_FONT_SIZE_MAX = 200
const FONT_SIZE_STEP = 10

// Outer fraction of the reading area that turns a page when tapped.
const TAP_EDGE = 0.3
const SWIPE_MIN_PX = 40

export function readerFormats(formats: readonly string[]): ReaderFormat[] {
  return READER_FORMATS.filter((f) => formats.includes(f))
}

/** The requested format when the book has it, else EPUB, else PDF. */
export function pickReaderFormat(
  formats: readonly string[],
  requested: string | null
): ReaderFormat | null {
  const available = readerFormats(formats)
  return available.find((f) => f === requested) ?? available[0] ?? null
}

/** Text and link colours for a theme, for engines that take them as settings. */
export function readerTextColors(theme: ReaderTheme): { fg: string; link: string } {
  const { fg, link } = THEME_COLORS[theme]
  return { fg, link }
}

export function readerBackground(theme: ReaderTheme): string {
  return THEME_COLORS[theme].bg
}

/** CSS injected into reflowable (EPUB) sections. */
export function readerStyles(theme: ReaderTheme, fontSize: number): string {
  const { bg, fg, link } = THEME_COLORS[theme]
  return `
    html {
      color-scheme: ${theme === 'dark' ? 'dark' : 'light'};
      font-size: ${fontSize}% !important;
    }
    html, body {
      background: ${bg} !important;
      color: ${fg} !important;
    }
    a:link, a:visited {
      color: ${link} !important;
    }
  `
}

/** Fixed-layout pages (PDF) are canvases, so they are tinted rather than restyled. */
export function fixedLayoutFilter(theme: ReaderTheme): string {
  if (theme === 'dark') return 'invert(0.9) hue-rotate(180deg)'
  if (theme === 'sepia') return 'sepia(0.4)'
  return 'none'
}

export function stepFontSize(current: number, direction: 1 | -1): number {
  const next = current + direction * FONT_SIZE_STEP
  return Math.min(READER_FONT_SIZE_MAX, Math.max(READER_FONT_SIZE_MIN, next))
}

/** Page turn for a tap at window x over a reading area starting at `left`. */
export function tapDirection(x: number, left: number, width: number): PageTurn | null {
  // A zero width gives NaN or ±Infinity, which the range check rejects.
  const fraction = (x - left) / width
  if (fraction < 0 || fraction > 1) return null
  if (fraction < TAP_EDGE) return 'left'
  if (fraction > 1 - TAP_EDGE) return 'right'
  return null
}

/** Page turn for a horizontal swipe; a leftward swipe reveals the page on the right. */
export function swipeDirection(dx: number, dy: number): PageTurn | null {
  if (Math.abs(dy) > Math.abs(dx)) return null
  if (dx <= -SWIPE_MIN_PX) return 'right'
  if (dx >= SWIPE_MIN_PX) return 'left'
  return null
}
