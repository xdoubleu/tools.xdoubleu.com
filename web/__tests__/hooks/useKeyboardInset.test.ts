import { renderHook, act } from '@testing-library/react'
import { useKeyboardInset } from '@/hooks/useKeyboardInset'

class MockVisualViewport extends EventTarget {
  height = window.innerHeight
  offsetTop = 0

  set(height: number, offsetTop = 0) {
    this.height = height
    this.offsetTop = offsetTop
    this.dispatchEvent(new Event('resize'))
  }
}

function installViewport() {
  const viewport = new MockVisualViewport()
  Object.defineProperty(window, 'visualViewport', { value: viewport, configurable: true })
  return viewport
}

afterEach(() => {
  Reflect.deleteProperty(window, 'visualViewport')
})

describe('useKeyboardInset', () => {
  it('stays 0 when visualViewport is unsupported', () => {
    Object.defineProperty(window, 'visualViewport', { value: null, configurable: true })
    const { result } = renderHook(() => useKeyboardInset())
    expect(result.current).toBe(0)
  })

  it('tracks the height the keyboard covers', () => {
    const viewport = installViewport()
    const { result } = renderHook(() => useKeyboardInset())
    expect(result.current).toBe(0)

    act(() => viewport.set(window.innerHeight - 300))
    expect(result.current).toBe(300)

    act(() => viewport.set(window.innerHeight))
    expect(result.current).toBe(0)
  })

  it('accounts for a panned visual viewport on scroll', () => {
    const viewport = installViewport()
    const { result } = renderHook(() => useKeyboardInset())

    act(() => {
      viewport.height = window.innerHeight - 300
      viewport.offsetTop = 100
      viewport.dispatchEvent(new Event('scroll'))
    })
    expect(result.current).toBe(200)
  })

  it('never goes negative', () => {
    const viewport = installViewport()
    const { result } = renderHook(() => useKeyboardInset())

    act(() => viewport.set(window.innerHeight + 50))
    expect(result.current).toBe(0)
  })

  it('stops listening on unmount', () => {
    const viewport = installViewport()
    const remove = jest.spyOn(viewport, 'removeEventListener')
    const { unmount } = renderHook(() => useKeyboardInset())

    unmount()
    expect(remove).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(remove).toHaveBeenCalledWith('scroll', expect.any(Function))
  })
})
