import { create } from '@bufbuild/protobuf'
import {
  GetSeriesResponseSchema,
  LibraryResponseSchema,
  type GetSeriesResponse
} from '@/lib/gen/books/v1/library_pb'
import {
  buildSeriesSummaries,
  formatSeriesPosition,
  parseSeriesPosition,
  seriesDescription,
  seriesHref,
  seriesLabel
} from '@/lib/books/series'

const ub = (id: string, status: string, seriesName: string, seriesTotal = 0) => ({
  id,
  bookId: id,
  status,
  book: { id, title: id, seriesName, seriesTotal }
})

describe('series helpers', () => {
  it('formats labels and links', () => {
    expect(seriesHref('Discworld: Death')).toBe('/books/series/Discworld%3A%20Death')
    expect(formatSeriesPosition(undefined)).toBe('')
    expect(formatSeriesPosition(0)).toBe('#0')
    expect(formatSeriesPosition(1.5)).toBe('#1.5')
    expect(seriesLabel({ seriesName: '', seriesPosition: 1 })).toBe('')
    expect(seriesLabel({ seriesName: 'Discworld', seriesPosition: 4 })).toBe('Discworld #4')
    expect(seriesLabel({ seriesName: 'Discworld', seriesPosition: undefined })).toBe('Discworld')
  })

  it('summarizes each series across the library', () => {
    const library = create(LibraryResponseSchema, {
      reading: [ub('a', 'currently-reading', 'Saga')],
      finished: [ub('b', 'read', 'Saga', 7), ub('c', 'read', 'Alpha')],
      wishlist: [ub('d', 'to-read', '')],
      shelves: [{ name: 'zines', books: [ub('e', 'zines', 'Alpha')] }]
    })
    expect(buildSeriesSummaries(library)).toEqual([
      { name: 'Alpha', owned: 2, read: 1, total: 2 },
      { name: 'Saga', owned: 2, read: 1, total: 7 }
    ])
    expect(buildSeriesSummaries(null)).toEqual([])
  })

  it('parses the edit form position', () => {
    expect(parseSeriesPosition('', '3')).toBeUndefined()
    expect(parseSeriesPosition('  ', '3')).toBeUndefined()
    expect(parseSeriesPosition('Saga', ' ')).toBeUndefined()
    expect(parseSeriesPosition('Saga', '-1')).toBeUndefined()
    expect(parseSeriesPosition('Saga', 'abc')).toBeUndefined()
    expect(parseSeriesPosition('Saga', '0')).toBe(0)
    expect(parseSeriesPosition('Saga', '2.5')).toBe(2.5)
  })

  it('describes a series page', () => {
    const series = (total: number, statuses: (string | null)[]): GetSeriesResponse =>
      create(GetSeriesResponseSchema, {
        name: 'Saga',
        total,
        entries: statuses.map((s, i) =>
          s === null
            ? { position: i, external: { title: `x${i}` } }
            : { position: i, userBook: { id: `u${i}`, status: s } }
        )
      })
    expect(seriesDescription(series(7, ['read', 'to-read', null]))).toBe(
      '1 of 7 read · 2 in your library · 1 missing'
    )
    expect(seriesDescription(series(0, ['read']))).toBe('1 of 1 read · 1 in your library')
  })
})
