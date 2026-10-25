import { act, render, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import {
  createMemoryRouter,
  MemoryRouter,
  Outlet,
  Route,
  RouterProvider,
  Routes,
  ScrollRestoration,
  useLocation
} from 'react-router'

vi.unmock('@/hooks/navigation')
const { usePathname, useRouter, useRouteParams, useSearchParams } =
  await vi.importActual<typeof import('@/hooks/navigation')>('@/hooks/navigation')

function wrapper(path: string, route = '*') {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={route} element={children} />
        </Routes>
      </MemoryRouter>
    )
  }
}

describe('navigation hooks', () => {
  it('reads the pathname', () => {
    const { result } = renderHook(() => usePathname(), { wrapper: wrapper('/books/library?q=1') })

    expect(result.current).toBe('/books/library')
  })

  it('reads the search params', () => {
    const { result } = renderHook(() => useSearchParams(), {
      wrapper: wrapper('/movies?list=shared')
    })

    expect(result.current.get('list')).toBe('shared')
  })

  it('reads the route params', () => {
    const { result } = renderHook(() => useRouteParams<{ id: string }>(), {
      wrapper: wrapper('/books/42', '/books/:id')
    })

    expect(result.current.id).toBe('42')
  })

  it('pushes, replaces and goes back', () => {
    const { result } = renderHook(() => ({ router: useRouter(), location: useLocation() }), {
      wrapper: wrapper('/start')
    })

    act(() => result.current.router.push('/a'))
    expect(result.current.location.pathname).toBe('/a')

    act(() => result.current.router.replace('/b'))
    expect(result.current.location.pathname).toBe('/b')

    act(() => result.current.router.back())
    expect(result.current.location.pathname).toBe('/start')
  })

  it('keeps one router object across navigations in a data router', () => {
    const seen: unknown[] = []
    function Probe() {
      useLocation()
      seen.push(useRouter())
      return <Outlet />
    }
    const router = createMemoryRouter(
      [
        {
          Component: Probe,
          children: [
            { path: '/start', element: null },
            { path: '/a', element: null }
          ]
        }
      ],
      { initialEntries: ['/start'] }
    )
    render(<RouterProvider router={router} />)

    act(() => {
      void router.navigate('/a')
    })

    expect(seen.length).toBeGreaterThan(1)
    expect(new Set(seen).size).toBe(1)
  })

  it('opens a pushed page at the top but keeps the position on replace', async () => {
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    let nav: ReturnType<typeof useRouter> | undefined
    function Probe() {
      nav = useRouter()
      return <ScrollRestoration />
    }
    const router = createMemoryRouter([{ path: '*', Component: Probe }], {
      initialEntries: ['/books/library']
    })
    render(<RouterProvider router={router} />)

    // A search box rewriting ?q= must not jump the list to the top.
    scrollTo.mockClear()
    await act(async () => nav!.replace('/books/library?q=dune'))
    expect(router.state.location.search).toBe('?q=dune')
    expect(scrollTo).not.toHaveBeenCalled()

    await act(async () => nav!.push('/books/42'))
    expect(scrollTo).toHaveBeenLastCalledWith(0, 0)
  })
})
