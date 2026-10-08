import {
  NEW_SEASONS,
  backlogRequest,
  isNewSeason,
  isUnreleased,
  mediaTypeLabel,
  monthLabel,
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

describe('backlogRequest', () => {
  it('passes a stored status through', () => {
    expect(
      backlogRequest({
        status: 'watched',
        mediaType: 'series',
        sort: 'title',
        onMyServices: true
      })
    ).toEqual({
      status: 'watched',
      mediaType: 'series',
      sort: 'title',
      newSeason: false,
      onMyServices: true
    })
  })

  it('turns the New seasons tab into the new-season filter', () => {
    expect(
      backlogRequest({ status: NEW_SEASONS, mediaType: '', sort: 'added', onMyServices: false })
    ).toEqual({
      status: '',
      mediaType: '',
      sort: 'added',
      newSeason: true,
      onMyServices: false
    })
  })
})

describe('isNewSeason', () => {
  const season = { number: 2, aired: true, watchedAt: [] as string[] }

  it('is an aired, counted, unticked season of a watched series', () => {
    expect(isNewSeason('watched', season)).toBe(true)
  })

  it.each([
    ['not watched', 'watching', season],
    ['specials', 'watched', { ...season, number: 0 }],
    ['unaired', 'watched', { ...season, aired: false }],
    ['ticked', 'watched', { ...season, watchedAt: [''] }]
  ])('is not when %s', (_name, status, s) => {
    expect(isNewSeason(status, s)).toBe(false)
  })
})

describe('monthLabel', () => {
  it('names every month', () => {
    const months = Array.from({ length: 12 }, (_, i) =>
      monthLabel(`2026-${String(i + 1).padStart(2, '0')}`)
    )
    expect(months.join(' ')).toBe(
      'Jan 26 Feb 26 Mar 26 Apr 26 May 26 Jun 26 Jul 26 Aug 26 Sep 26 Oct 26 Nov 26 Dec 26'
    )
    expect(monthLabel('2025-12')).toBe('Dec 25')
  })
})
