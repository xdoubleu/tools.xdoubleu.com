/** "1h 2m" or "45 min"; empty when the feed gave no duration. */
export function durationLabel(seconds: number | undefined): string {
  if (seconds === undefined || seconds <= 0) return ''
  const minutes = Math.max(1, Math.round(seconds / 60))
  if (minutes < 60) return `${minutes} min`
  const rest = minutes % 60
  return rest === 0 ? `${Math.floor(minutes / 60)}h` : `${Math.floor(minutes / 60)}h ${rest}m`
}
