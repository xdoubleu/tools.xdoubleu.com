export function oneYearAgo(): string {
  const d = new Date()
  d.setFullYear(d.getFullYear() - 1)
  return d.toISOString().slice(0, 10)
}

export function today(): string {
  return new Date().toISOString().slice(0, 10)
}

/** Formats a date as dd/MM/yyyy. Returns '' for empty or unparseable input. */
export function formatDate(value: string | Date): string {
  if (!value) return ''
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value)) {
    // Reorder date-only strings directly: new Date('yyyy-MM-dd') is UTC
    // midnight and shifts a day in negative-offset timezones.
    const [y, m, d] = value.split('-')
    return `${d}/${m}/${y}`
  }
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString('en-GB')
}

/** Formats a timestamp as dd/MM/yyyy, HH:mm:ss. Returns '' for empty or unparseable input. */
export function formatDateTime(value: string | Date): string {
  if (!value) return ''
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('en-GB')
}
