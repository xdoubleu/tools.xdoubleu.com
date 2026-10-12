import type { ReadSuggestion } from '@/lib/gen/books/v1/library_pb'

export interface ReadSuggestionText {
  /** "Best on iPad/laptop (PDF)" or "Best on phone/Kobo (EPUB)". */
  label: string
  /** The figure count behind the suggestion. */
  detail: string
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`

/**
 * Describes the server's figure-density suggestion. A figure-heavy book without
 * a PDF names only the device. Pages are estimated when no PDF was measured.
 */
export function readSuggestionText(
  suggestion: ReadSuggestion | undefined,
  formats: string[]
): ReadSuggestionText | null {
  if (!suggestion) return null
  const hasPdf = formats.includes('pdf')
  let label = 'Best on phone/Kobo (EPUB)'
  if (suggestion.format === 'pdf') {
    label = hasPdf ? 'Best on iPad/laptop (PDF)' : 'Best on iPad/laptop'
  }
  const pages = plural(suggestion.pages, 'page')
  const detail = `${plural(suggestion.figureCount, 'figure')} in ${hasPdf ? '' : '~'}${pages}`
  return { label, detail }
}
