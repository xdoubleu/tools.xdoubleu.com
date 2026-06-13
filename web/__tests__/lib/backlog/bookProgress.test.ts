import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'
import { displayProgressPercent } from '@/lib/backlog/bookProgress'

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
