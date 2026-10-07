export const STATUSES = ['want', 'watching', 'watched', 'dropped'] as const
export type Status = (typeof STATUSES)[number]

export interface BacklogFilter {
  status: string
  mediaType: string
  sort: string
}

export const DEFAULT_BACKLOG_FILTER: BacklogFilter = { status: '', mediaType: '', sort: 'added' }

export const STATUS_LABELS: Record<Status, string> = {
  want: 'Want',
  watching: 'Watching',
  watched: 'Watched',
  dropped: 'Dropped'
}

export function statusLabel(status: string): string {
  return (STATUS_LABELS as Record<string, string>)[status] ?? status
}

export function mediaTypeLabel(mediaType: string): string {
  return mediaType === 'series' ? 'Series' : 'Movie'
}

/** TMDB poster URL at a fixed width; empty when the title has none. */
export function posterUrl(posterPath: string, width: 'w92' | 'w185' | 'w342' = 'w185'): string {
  return posterPath ? `https://image.tmdb.org/t/p/${width}${posterPath}` : ''
}

/** Whether a YYYY-MM-DD release date lies after `today`; "" never does. */
export function isUnreleased(releaseDate: string, today: string): boolean {
  return releaseDate > today
}

/** "1999", "Releases 2027-05-01", or "" when TMDB has no date. */
export function releaseLabel(releaseDate: string, today: string): string {
  return isUnreleased(releaseDate, today) ? `Releases ${releaseDate}` : releaseDate.slice(0, 4)
}

export function todayISO(now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/** A watch's day as YYYY-MM-DD in local time; '' for an unknown date. */
export function watchDay(watchedAt: string): string {
  return watchedAt ? todayISO(new Date(watchedAt)) : ''
}
