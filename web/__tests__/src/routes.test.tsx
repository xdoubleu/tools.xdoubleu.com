import { render, waitFor } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { pageRoutes, redirectRoutes, routePath } from '@/src/routes'

describe('routePath', () => {
  it.each([
    ['../app/page.tsx', ''],
    ['../app/feeds/page.tsx', 'feeds'],
    ['../app/books/[id]/read/page.tsx', 'books/:id/read'],
    [
      '../app/books/external/[provider]/[providerId]/page.tsx',
      'books/external/:provider/:providerId'
    ],
    ['../app/dashboard/games/[token]/page.tsx', 'dashboard/games/:token']
  ])('maps %s to %j', (file, path) => {
    expect(routePath(file)).toBe(path)
  })
})

describe('pageRoutes', () => {
  it('loads each page lazily as the route component', async () => {
    const Page = () => null
    const routes = pageRoutes({
      '../app/trains/[journeyId]/page.tsx': async () => ({ default: Page })
    })

    expect(routes).toHaveLength(1)
    expect(routes[0].path).toBe('trains/:journeyId')
    const { lazy } = routes[0]
    if (typeof lazy !== 'function') throw new Error('expected a lazy route')
    expect(await lazy()).toEqual({ Component: Page })
  })
})

describe('redirectRoutes', () => {
  it.each([
    ['/books/feed', '/feeds'],
    ['/learningpaths', '/learningpaths/list'],
    ['/recipes', '/recipes/list']
  ])('sends %s on to %s', async (from, to) => {
    const router = createMemoryRouter([...redirectRoutes, { path: '*', Component: () => null }], {
      initialEntries: [from]
    })
    render(<RouterProvider router={router} />)
    await waitFor(() => expect(router.state.location.pathname).toBe(to))
  })
})
