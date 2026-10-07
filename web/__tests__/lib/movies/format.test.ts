import {
  isUnreleased,
  mediaTypeLabel,
  posterUrl,
  releaseLabel,
  statusLabel,
  todayISO,
  watchDay
} from '@/lib/movies/format'

describe('movies format helpers', () => {
  it('labels statuses and media types', () => {
    expect(['want', 'watching', 'watched', 'dropped'].map(statusLabel)).toEqual([
      'Want',
      'Watching',
      'Watched',
      'Dropped'
    ])
    expect(statusLabel('unknown')).toBe('unknown')
    expect(mediaTypeLabel('series')).toBe('Series')
    expect(mediaTypeLabel('movie')).toBe('Movie')
  })

  it('builds TMDB poster URLs', () => {
    expect(posterUrl('/a.jpg')).toBe('https://image.tmdb.org/t/p/w185/a.jpg')
    expect(posterUrl('/a.jpg', 'w92')).toBe('https://image.tmdb.org/t/p/w92/a.jpg')
    expect(posterUrl('')).toBe('')
  })

  it('labels release dates relative to today', () => {
    expect(releaseLabel('1999-03-30', '2026-10-07')).toBe('1999')
    expect(releaseLabel('2027-05-01', '2026-10-07')).toBe('Releases 2027-05-01')
    expect(releaseLabel('', '2026-10-07')).toBe('')
    expect(isUnreleased('', '2026-10-07')).toBe(false)
    expect(releaseLabel('2026-10-07', '2026-10-07')).toBe('2026')
  })

  it('formats dates in local time', () => {
    expect(todayISO(new Date(2026, 0, 5))).toBe('2026-01-05')
    expect(watchDay(new Date(2026, 9, 7, 12).toISOString())).toBe('2026-10-07')
    expect(watchDay('')).toBe('')
  })
})
