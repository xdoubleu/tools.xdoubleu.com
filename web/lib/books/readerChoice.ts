// Which file the reader opens: the original EPUB/PDF or its converted KEPUB.
// Remembered per book on this device.
import type { ReaderFormat } from './readerSettings'

export type ReaderChoice = 'original' | 'kepub'

/** A format switch for the reader's settings. */
export interface ReaderChoiceControl {
  value: ReaderChoice
  /** The original file's format, for its label. */
  original: ReaderFormat
  onChange: (choice: ReaderChoice) => void
}

/** localStorage key; the value is the raw choice string. */
export function readerChoiceKey(bookId: string): string {
  return `books:reader-choice:${bookId}`
}

function isReaderChoice(value: string | null): value is ReaderChoice {
  return value === 'original' || value === 'kepub'
}

/** This device's choice for the book, or null when none is stored. */
export function loadReaderChoice(bookId: string): ReaderChoice | null {
  try {
    const value = localStorage.getItem(readerChoiceKey(bookId))
    return isReaderChoice(value) ? value : null
  } catch {
    return null
  }
}

export function saveReaderChoice(bookId: string, choice: ReaderChoice): void {
  try {
    localStorage.setItem(readerChoiceKey(bookId), choice)
  } catch {
    // Unavailable storage (private browsing, quota): the choice lasts this visit.
  }
}

/** The choice a `?format=` value implies, if any. */
export function requestedReaderChoice(requested: string | null): ReaderChoice | null {
  if (requested === 'kepub') return 'kepub'
  return requested === 'epub' || requested === 'pdf' ? 'original' : null
}

/** The format to fetch for a book whose original file is `original`. */
export function readerFileFormat(
  original: ReaderFormat,
  choice: ReaderChoice
): ReaderFormat | 'kepub' {
  return choice === 'kepub' ? 'kepub' : original
}

export function readerChoiceOptions(original: ReaderFormat) {
  return [
    { value: 'original' as const, label: `Original (${original.toUpperCase()})` },
    { value: 'kepub' as const, label: 'Converted (KEPUB)' }
  ]
}
