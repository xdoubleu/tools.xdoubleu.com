import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'
import {
  displayProgressPercent,
  defaultProgressMode,
  PROGRESS_MODE_PAGES,
  PROGRESS_MODE_PERCENT
} from '@/lib/books/bookProgress'

function userBook(fields: Parameters<typeof create<typeof UserBookSchema>>[1]) {
  return create(UserBookSchema, fields)
}

describe('displayProgressPercent', () => {
  it('returns the stored percent in percent mode', () => {
    expect(displayProgressPercent(userBook({ progressMode: 'percent', progressPercent: 60 }))).toBe(
      60
    )
  })

  it('clamps the percent above 100', () => {
    expect(
      displayProgressPercent(userBook({ progressMode: 'percent', progressPercent: 140 }))
    ).toBe(100)
  })

  it('derives the percent from pages', () => {
    expect(
      displayProgressPercent(
        userBook({
          progressMode: 'pages',
          currentPage: 150,
          book: create(BookSchema, { pageCount: 300 })
        })
      )
    ).toBe(50)
  })

  it('returns zero when the page count is unknown', () => {
    expect(
      displayProgressPercent(
        userBook({ progressMode: 'pages', currentPage: 150, book: create(BookSchema, {}) })
      )
    ).toBe(0)
  })

  it('returns zero when there is no book', () => {
    expect(displayProgressPercent(userBook({ progressMode: 'pages', currentPage: 10 }))).toBe(0)
  })
})

describe('defaultProgressMode', () => {
  it('returns the stored mode when already set', () => {
    expect(defaultProgressMode(userBook({ progressMode: PROGRESS_MODE_PERCENT }))).toBe(
      PROGRESS_MODE_PERCENT
    )
    expect(defaultProgressMode(userBook({ progressMode: PROGRESS_MODE_PAGES }))).toBe(
      PROGRESS_MODE_PAGES
    )
  })

  it('defaults to percent for digital-only books', () => {
    expect(defaultProgressMode(userBook({ tags: ['own-digital'] }))).toBe(PROGRESS_MODE_PERCENT)
  })

  it('defaults to pages for physical books', () => {
    expect(defaultProgressMode(userBook({ tags: ['own-physical'] }))).toBe(PROGRESS_MODE_PAGES)
  })

  it('defaults to pages for books with both physical and digital', () => {
    expect(defaultProgressMode(userBook({ tags: ['own-physical', 'own-digital'] }))).toBe(
      PROGRESS_MODE_PAGES
    )
  })

  it('defaults to pages for books with no ownership tags', () => {
    expect(defaultProgressMode(userBook({}))).toBe(PROGRESS_MODE_PAGES)
  })

  it('respects stored mode over tag defaults', () => {
    // Has stored mode "percent" even though it is a physical book.
    expect(
      defaultProgressMode(userBook({ progressMode: PROGRESS_MODE_PERCENT, tags: ['own-physical'] }))
    ).toBe(PROGRESS_MODE_PERCENT)
  })
})
