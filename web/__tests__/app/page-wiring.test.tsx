import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { useRouteParams, useSearchParams } from '@/hooks/navigation'
import { useSharedLibrary, useSharedSteam } from '@/hooks/useDashboardShare'
import { useSharedList } from '@/hooks/useSharedList'

vi.mock('@/hooks/useDashboardShare', () => ({
  useSharedSteam: vi.fn(),
  useSharedLibrary: vi.fn()
}))
vi.mock('@/hooks/useSharedList', () => ({ useSharedList: vi.fn() }))
vi.mock('@/components/dashboard/GamesDashboardPublicClient', () => ({
  default: ({ token }: { token: string }) => <div data-testid="games-client">{token}</div>
}))
vi.mock('@/components/dashboard/GamesDashboardPublicLibrary', () => ({
  default: ({ token }: { token: string }) => <div data-testid="games-library">{token}</div>
}))
vi.mock('@/components/dashboard/ReadingDashboardPublicClient', () => ({
  default: ({ token }: { token: string }) => <div data-testid="reading-client">{token}</div>
}))
vi.mock('@/components/dashboard/ReadingDashboardPublicLibrary', () => ({
  default: ({ token }: { token: string }) => <div data-testid="reading-library">{token}</div>
}))
vi.mock('@/components/dashboard/GamesDashboardPublicGameClient', () => ({
  default: ({ token, id }: { token: string; id: string }) => (
    <div data-testid="game-client">{`${token}/${id}`}</div>
  )
}))
vi.mock('@/components/feeds/FeedFilteredItemsClient', () => ({
  default: ({ initialFeedId }: { initialFeedId?: string }) => (
    <div data-testid="filtered">{initialFeedId ?? 'all'}</div>
  )
}))
vi.mock('@/components/movies/MoviesStatsClient', () => ({ default: () => <i /> }))

import GamesPublicPage from '@/app/dashboard/games/[token]/page'
import GamesPublicLibraryPage from '@/app/dashboard/games/[token]/library/page'
import GamesPublicGamePage from '@/app/dashboard/games/[token]/[id]/page'
import ReadingPublicPage from '@/app/dashboard/reading/[token]/page'
import ReadingPublicLibraryPage from '@/app/dashboard/reading/[token]/library/page'
import FeedFilteredPage from '@/app/feeds/filtered/page'
import MoviesStatsPage from '@/app/movies/stats/page'

function mount(page: React.ReactNode) {
  return render(<MemoryRouter>{page}</MemoryRouter>)
}

function sharedWithName(hook: typeof useSharedSteam | typeof useSharedLibrary, name?: string) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only data is read
  vi.mocked(hook).mockReturnValue({ data: name ? { displayName: name } : undefined } as never)
}

describe('shared games pages', () => {
  beforeEach(() => vi.mocked(useRouteParams).mockReturnValue({ token: 'tok-1', id: '730' }))

  it('names the dashboard after the owner once it has loaded', () => {
    sharedWithName(useSharedSteam, 'Bob')
    mount(<GamesPublicPage />)

    expect(screen.getByRole('heading', { name: "Bob's games" })).toBeInTheDocument()
    expect(screen.getByTestId('games-client')).toHaveTextContent('tok-1')
  })

  it('uses a generic heading while the link is loading or invalid', () => {
    sharedWithName(useSharedSteam)
    mount(<GamesPublicPage />)

    expect(screen.getByRole('heading', { name: 'Shared games' })).toBeInTheDocument()
  })

  it('links the library breadcrumb back to the owner dashboard', () => {
    sharedWithName(useSharedSteam, 'Bob')
    mount(<GamesPublicLibraryPage />)

    expect(screen.getByRole('link', { name: "Bob's games" })).toHaveAttribute(
      'href',
      '/dashboard/games/tok-1'
    )
    expect(screen.getByTestId('games-library')).toHaveTextContent('tok-1')
  })

  it('falls back to a generic breadcrumb', () => {
    sharedWithName(useSharedSteam)
    mount(<GamesPublicLibraryPage />)

    expect(screen.getByRole('link', { name: 'Games' })).toBeInTheDocument()
  })

  it('passes the token and game id to the game client', () => {
    mount(<GamesPublicGamePage />)

    expect(screen.getByTestId('game-client')).toHaveTextContent('tok-1/730')
  })
})

describe('shared reading pages', () => {
  beforeEach(() => vi.mocked(useRouteParams).mockReturnValue({ token: 'tok-2' }))

  it('names the dashboard after the owner once it has loaded', () => {
    sharedWithName(useSharedLibrary, 'Eve')
    mount(<ReadingPublicPage />)

    expect(screen.getByRole('heading', { name: "Eve's reading" })).toBeInTheDocument()
    expect(screen.getByTestId('reading-client')).toHaveTextContent('tok-2')
  })

  it('uses a generic heading without an owner name', () => {
    sharedWithName(useSharedLibrary)
    mount(<ReadingPublicPage />)

    expect(screen.getByRole('heading', { name: 'Shared reading' })).toBeInTheDocument()
  })

  it('links the library breadcrumb back to the owner dashboard', () => {
    sharedWithName(useSharedLibrary, 'Eve')
    mount(<ReadingPublicLibraryPage />)

    expect(screen.getByRole('link', { name: "Eve's reading" })).toHaveAttribute(
      'href',
      '/dashboard/reading/tok-2'
    )
    expect(screen.getByTestId('reading-library')).toHaveTextContent('tok-2')
  })

  it('falls back to a generic breadcrumb', () => {
    sharedWithName(useSharedLibrary)
    mount(<ReadingPublicLibraryPage />)

    expect(screen.getByRole('link', { name: 'Reading' })).toBeInTheDocument()
  })
})

describe('FeedFilteredPage', () => {
  it('filters to the feed named in the query', () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams('feed=f-9'))
    mount(<FeedFilteredPage />)

    expect(screen.getByTestId('filtered')).toHaveTextContent('f-9')
  })

  it('shows every feed without one', () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams())
    mount(<FeedFilteredPage />)

    expect(screen.getByTestId('filtered')).toHaveTextContent('all')
  })
})

describe('MoviesStatsPage', () => {
  it.each([
    [false, '/movies'],
    [true, '/movies?list=shared']
  ])('keeps the shared list (%s) in the breadcrumb', (shared, href) => {
    vi.mocked(useSharedList).mockReturnValue(shared)
    mount(<MoviesStatsPage />)

    expect(screen.getByRole('link', { name: 'Movies & Series' })).toHaveAttribute('href', href)
  })
})
