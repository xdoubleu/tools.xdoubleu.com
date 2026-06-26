import { renderHook, act } from '@testing-library/react'
import { useLocalStorage } from '@/hooks/useLocalStorage'

beforeEach(() => {
  localStorage.clear()
})

describe('useLocalStorage', () => {
  it('returns the initial value when nothing is stored', () => {
    const { result } = renderHook(() => useLocalStorage('test:key', 42))
    expect(result.current[0]).toBe(42)
  })

  it('updates the stored value when the setter is called', () => {
    const { result } = renderHook(() => useLocalStorage('test:key', 0))
    act(() => {
      result.current[1](99)
    })
    expect(result.current[0]).toBe(99)
    expect(JSON.parse(localStorage.getItem('test:key') ?? 'null')).toBe(99)
  })

  it('reads a pre-existing value from localStorage on mount', () => {
    localStorage.setItem('test:key', JSON.stringify('hello'))
    const { result } = renderHook(() => useLocalStorage('test:key', 'default'))
    // useEffect fires during render in RTL
    expect(result.current[0]).toBe('hello')
  })

  it('works with array values', () => {
    const { result } = renderHook(() => useLocalStorage<string[]>('test:arr', []))
    act(() => {
      result.current[1](['a', 'b'])
    })
    expect(result.current[0]).toEqual(['a', 'b'])
    expect(JSON.parse(localStorage.getItem('test:arr') ?? 'null')).toEqual(['a', 'b'])
  })

  it('persists across remounts with the same key', () => {
    const { result: r1 } = renderHook(() => useLocalStorage('test:persist', 0))
    act(() => {
      r1.current[1](7)
    })

    const { result: r2 } = renderHook(() => useLocalStorage('test:persist', 0))
    expect(r2.current[0]).toBe(7)
  })
})
