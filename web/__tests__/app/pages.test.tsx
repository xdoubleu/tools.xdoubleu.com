import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useRouteParams } from '@/hooks/navigation'

// Every route renders without throwing while its data is still loading. Pages
// are thin compositions of the clients tested elsewhere, so this guards the
// wiring (imports, params, router hooks) rather than behavior.
const pages = import.meta.glob<{ default: React.ComponentType }>('../../app/**/page.tsx')

const pending = () => new Proxy({}, { get: () => () => new Promise(() => {}) })

vi.mock('@/lib/client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/client')>()),
  createServiceClient: () => pending(),
  createKeepaliveClient: () => pending()
}))

// Live WebRTC rooms: covered by RoomClients.test.tsx.
vi.mock('@/app/watchparty/[id]/ViewerClient', () => ({ default: () => null }))
vi.mock('@/app/watchparty/[id]/presenter/PresenterClient', () => ({ default: () => null }))

// Pages that need a live session are covered by their own tests.
const OWN_TESTS = ['app/oauth/consent/page.tsx']

describe('pages', () => {
  beforeEach(() => {
    vi.mocked(useRouteParams).mockReturnValue(
      new Proxy({}, { get: (_, key) => (typeof key === 'string' ? `${key}-1` : undefined) })
    )
    vi.stubGlobal('fetch', () => new Promise(() => {}))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  const files = Object.keys(pages).filter((f) => !OWN_TESTS.some((o) => f.endsWith(o)))

  it('finds every page', () => {
    expect(files.length).toBeGreaterThan(50)
  })

  it.each(files)('%s renders', async (file) => {
    const { default: Page } = await pages[file]()

    const { unmount } = render(
      <MemoryRouter>
        <Page />
      </MemoryRouter>
    )

    unmount()
  })
})
