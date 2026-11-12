import { act, renderHook } from '@testing-library/react'
import { useScrolledTitle } from '@/hooks/useScrolledTitle'

/** A rect with only the edge the hook reads. */
const rectAt = (bottom: number) => () => ({ ...new DOMRect(), bottom })

function mountHeading(html: string, bottom: number) {
  document.body.innerHTML = `<main>${html}</main>`
  const h1 = document.querySelector('main h1:not(.sr-only)') ?? document.querySelector('h1')
  if (h1) h1.getBoundingClientRect = rectAt(bottom)
  return h1
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('useScrolledTitle', () => {
  it('is null while the heading is in view', () => {
    mountHeading('<h1>Recipes</h1>', 120)
    const { result } = renderHook(() => useScrolledTitle('/recipes'))
    expect(result.current).toBeNull()
  })

  it('returns the heading text once it is under the bar', () => {
    mountHeading('<h1> Recipes </h1>', 20)
    const { result } = renderHook(() => useScrolledTitle('/recipes'))
    expect(result.current).toBe('Recipes')
  })

  it('treats a heading exactly at the bar edge as still visible', () => {
    mountHeading('<h1>Recipes</h1>', 56)
    const { result } = renderHook(() => useScrolledTitle('/recipes'))
    expect(result.current).toBeNull()
  })

  it('follows the scroll position', () => {
    const h1 = mountHeading('<h1>Recipes</h1>', 120)
    const { result } = renderHook(() => useScrolledTitle('/recipes'))
    expect(result.current).toBeNull()
    act(() => {
      if (h1) h1.getBoundingClientRect = rectAt(10)
      window.dispatchEvent(new Event('scroll'))
    })
    expect(result.current).toBe('Recipes')
  })

  it('uses the bar height it is given', () => {
    mountHeading('<h1>Recipes</h1>', 70)
    const { result } = renderHook(() => useScrolledTitle('/recipes', 100))
    expect(result.current).toBe('Recipes')
  })

  it('ignores a screen-reader-only heading and a page without one', () => {
    mountHeading('<h1 class="sr-only">Hidden</h1>', 0)
    const { result, rerender } = renderHook(() => useScrolledTitle('/a'))
    expect(result.current).toBeNull()
    document.body.innerHTML = '<main><p>no heading</p></main>'
    rerender()
    act(() => {
      window.dispatchEvent(new Event('scroll'))
    })
    expect(result.current).toBeNull()
  })

  it('listens passively so scrolling is never blocked', () => {
    const add = vi.spyOn(window, 'addEventListener')
    renderHook(() => useScrolledTitle('/a'))
    expect(add).toHaveBeenCalledWith('scroll', expect.any(Function), { passive: true })
    add.mockRestore()
  })

  it('stops listening on unmount', () => {
    const remove = vi.spyOn(window, 'removeEventListener')
    const { unmount } = renderHook(() => useScrolledTitle('/a'))
    unmount()
    expect(remove).toHaveBeenCalledWith('scroll', expect.any(Function))
    remove.mockRestore()
  })

  it('re-measures when the key changes', () => {
    mountHeading('<h1>One</h1>', 120)
    const { result, rerender } = renderHook(({ k }) => useScrolledTitle(k), {
      initialProps: { k: '/one' }
    })
    expect(result.current).toBeNull()
    mountHeading('<h1>Two</h1>', 5)
    rerender({ k: '/two' })
    expect(result.current).toBe('Two')
  })
})
