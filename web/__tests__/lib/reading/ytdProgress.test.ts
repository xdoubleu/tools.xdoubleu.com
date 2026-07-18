import { create } from '@bufbuild/protobuf'
import { UserBookSchema } from '@/lib/gen/reading/v1/library_pb'
import { ytdProgress } from '@/lib/reading/ytdProgress'

const YEAR = 2026

function makeUserBook(finishedAt: string[]) {
  return create(UserBookSchema, { finishedAt })
}

describe('ytdProgress', () => {
  beforeEach(() => {
    jest.useFakeTimers().setSystemTime(new Date(`${YEAR}-06-13T12:00:00Z`))
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('returns empty series and zero total when no finished books', () => {
    expect(ytdProgress([])).toEqual({ series: [], total: 0 })
  })

  it('returns empty series when all finishes are from prior years', () => {
    const books = [makeUserBook([`${YEAR - 1}-03-01T00:00:00Z`])]
    expect(ytdProgress(books)).toEqual({ series: [], total: 0 })
  })

  it('counts a single finish event in the current year', () => {
    const books = [makeUserBook([`${YEAR}-02-10T00:00:00Z`])]
    const result = ytdProgress(books)
    expect(result.total).toBe(1)
    expect(result.series).toEqual([{ label: `${YEAR}-02-10`, value: 1 }])
  })

  it('counts re-read of the same book twice (two finishedAt entries)', () => {
    const books = [makeUserBook([`${YEAR}-01-15T00:00:00Z`, `${YEAR}-05-20T00:00:00Z`])]
    const result = ytdProgress(books)
    expect(result.total).toBe(2)
    expect(result.series).toEqual([
      { label: `${YEAR}-01-15`, value: 1 },
      { label: `${YEAR}-05-20`, value: 2 }
    ])
  })

  it('aggregates multiple books finished on the same day', () => {
    const books = [
      makeUserBook([`${YEAR}-03-05T00:00:00Z`]),
      makeUserBook([`${YEAR}-03-05T10:00:00Z`])
    ]
    const result = ytdProgress(books)
    expect(result.total).toBe(2)
    expect(result.series).toHaveLength(1)
    expect(result.series[0]).toEqual({ label: `${YEAR}-03-05`, value: 2 })
  })

  it('builds cumulative series in chronological order', () => {
    const books = [
      makeUserBook([`${YEAR}-04-01T00:00:00Z`]),
      makeUserBook([`${YEAR}-02-01T00:00:00Z`]),
      makeUserBook([`${YEAR}-06-01T00:00:00Z`])
    ]
    const result = ytdProgress(books)
    expect(result.total).toBe(3)
    expect(result.series).toEqual([
      { label: `${YEAR}-02-01`, value: 1 },
      { label: `${YEAR}-04-01`, value: 2 },
      { label: `${YEAR}-06-01`, value: 3 }
    ])
  })

  it('ignores prior-year finishes on books that also have a current-year finish', () => {
    const books = [makeUserBook([`${YEAR - 1}-12-20T00:00:00Z`, `${YEAR}-01-05T00:00:00Z`])]
    const result = ytdProgress(books)
    expect(result.total).toBe(1)
    expect(result.series).toEqual([{ label: `${YEAR}-01-05`, value: 1 }])
  })
})
