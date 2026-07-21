import { useState } from 'react'
import { oneYearAgo, today } from '@/lib/dates'

export interface DashboardChartState<V extends string> {
  view: V
  setView: (v: V) => void
  start: string
  setStart: (v: string) => void
  end: string
  setEnd: (v: string) => void
}

/**
 * Shared chart view + date-range state for the private and public dashboards.
 * Lives in the wrapper (the progress hook above the view needs start/end), so
 * there is one declaration site and the tab/date UI can't drift between them.
 */
export function useDashboardChartState<V extends string>(defaultView: V): DashboardChartState<V> {
  const [view, setView] = useState<V>(defaultView)
  const [start, setStart] = useState(oneYearAgo())
  const [end, setEnd] = useState(today())
  return { view, setView, start, setStart, end, setEnd }
}
