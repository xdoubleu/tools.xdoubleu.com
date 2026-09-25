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

/** Chart view + date-range state shared by the private and public dashboards. */
export function useDashboardChartState<V extends string>(defaultView: V): DashboardChartState<V> {
  const [view, setView] = useState<V>(defaultView)
  const [start, setStart] = useState(oneYearAgo())
  const [end, setEnd] = useState(today())
  return { view, setView, start, setStart, end, setEnd }
}
