import { renderHook, act } from '@testing-library/react'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { oneYearAgo, today } from '@/lib/dates'

describe('useDashboardChartState', () => {
  it('starts on the given view with a one-year default range', () => {
    const { result } = renderHook(() => useDashboardChartState<'ytd' | 'all'>('ytd'))
    expect(result.current.view).toBe('ytd')
    expect(result.current.start).toBe(oneYearAgo())
    expect(result.current.end).toBe(today())
  })

  it('updates the view and the date range through the setters', () => {
    const { result } = renderHook(() => useDashboardChartState<'ytd' | 'all'>('ytd'))
    act(() => {
      result.current.setView('all')
      result.current.setStart('2026-01-01')
      result.current.setEnd('2026-02-01')
    })
    expect(result.current.view).toBe('all')
    expect(result.current.start).toBe('2026-01-01')
    expect(result.current.end).toBe('2026-02-01')
  })
})
