import { createElement } from 'react'
import { renderToString } from 'react-dom/server'
import { act, renderHook } from '@testing-library/react'
import { useReaderChoice } from '@/hooks/useReaderChoice'

describe('useReaderChoice', () => {
  beforeEach(() => localStorage.clear())

  it('defaults to the original', () => {
    const { result } = renderHook(() => useReaderChoice('book-1', null))
    expect(result.current[0]).toBe('original')
  })

  it('reads this book’s stored choice', () => {
    localStorage.setItem('books:reader-choice:book-1', 'kepub')
    expect(renderHook(() => useReaderChoice('book-1', null)).result.current[0]).toBe('kepub')
    expect(renderHook(() => useReaderChoice('book-2', null)).result.current[0]).toBe('original')
  })

  it('lets ?format= override the stored choice', () => {
    localStorage.setItem('books:reader-choice:book-1', 'kepub')
    expect(renderHook(() => useReaderChoice('book-1', 'pdf')).result.current[0]).toBe('original')
    localStorage.setItem('books:reader-choice:book-1', 'original')
    expect(renderHook(() => useReaderChoice('book-1', 'kepub')).result.current[0]).toBe('kepub')
  })

  it('applies and stores a toggle over ?format=', () => {
    const { result } = renderHook(() => useReaderChoice('book-1', 'epub'))
    act(() => result.current[1]('kepub'))
    expect(result.current[0]).toBe('kepub')
    expect(localStorage.getItem('books:reader-choice:book-1')).toBe('kepub')
  })

  it('does not store a toggle before the book is known', () => {
    const { result } = renderHook(() => useReaderChoice(null, null))
    act(() => result.current[1]('kepub'))
    expect(result.current[0]).toBe('kepub')
    expect(localStorage.length).toBe(0)
  })

  it('is unknown while server-rendering, before storage can be read', () => {
    localStorage.setItem('books:reader-choice:book-1', 'kepub')
    function Probe() {
      return String(useReaderChoice('book-1', null)[0])
    }
    expect(renderToString(createElement(Probe))).toBe('undefined')
  })
})
