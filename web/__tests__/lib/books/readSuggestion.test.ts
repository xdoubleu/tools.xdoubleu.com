import { create } from '@bufbuild/protobuf'
import { ReadSuggestionSchema } from '@/lib/gen/books/v1/library_pb'
import { readSuggestionText } from '@/lib/books/readSuggestion'

describe('readSuggestionText', () => {
  it('returns null before the book is analysed', () => {
    expect(readSuggestionText(undefined, ['pdf'])).toBeNull()
  })

  it('suggests the PDF on a tablet for a figure-heavy book with a PDF', () => {
    const s = create(ReadSuggestionSchema, { format: 'pdf', figureCount: 42, pages: 310 })
    expect(readSuggestionText(s, ['epub', 'pdf'])).toEqual({
      label: 'Best on iPad/laptop (PDF)',
      detail: '42 figures in 310 pages'
    })
  })

  it('names only the device for a figure-heavy book without a PDF', () => {
    const s = create(ReadSuggestionSchema, { format: 'pdf', figureCount: 1, pages: 1 })
    expect(readSuggestionText(s, ['epub'])).toEqual({
      label: 'Best on iPad/laptop',
      detail: '1 figure in ~1 page'
    })
  })

  it('suggests the EPUB on a phone or Kobo for a text book', () => {
    const s = create(ReadSuggestionSchema, { format: 'epub', figureCount: 0, pages: 120 })
    expect(readSuggestionText(s, ['epub'])).toEqual({
      label: 'Best on phone/Kobo (EPUB)',
      detail: '0 figures in ~120 pages'
    })
  })
})
