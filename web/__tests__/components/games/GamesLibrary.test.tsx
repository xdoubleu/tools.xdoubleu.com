import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockRefresh = vi.fn()
const mockSetGameFavourite = vi.fn().mockResolvedValue({})

const mockReplace = vi.fn()
let mockShelf: string | null = null
let mockWishlistError: Error | undefined
let mockNoWishlistData = false
let mockWishlist = [{ id: 5, name: 'Hollow Knight', imageUrl: '' }]
vi.mock('@/hooks/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  useSearchParams: () => new URLSearchParams(mockShelf ? `shelf=${mockShelf}` : '')
}))

vi.mock('@/hooks/useGames', () => ({
  useSteam: vi.fn(),
  useWishlist: () => ({
    data: mockNoWishlistData ? undefined : { games: mockWishlist },
    error: mockWishlistError
  }),
  useSetGameFavourite: () => mockSetGameFavourite
}))

vi.mock('@/lib/games/steamRefresh', () => ({
  useSteamRefresh: vi.fn()
}))

vi.mock('swr', () => ({ mutate: vi.fn() }))
import { mutate } from 'swr'

import GamesLibrary from '@/components/games/GamesLibrary'
import { useSteam } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  SteamResponseSchema,
  GetSteamResponseSchema
} from '@/lib/gen/games/v1/games_pb'

const mockUseBacklogSteam = vi.mocked(useSteam)
const mockUseSteamRefresh = vi.mocked(useSteamRefresh)

function mockRefreshState(overrides: Partial<ReturnType<typeof useSteamRefresh>> = {}) {
  mockUseSteamRefresh.mockReturnValue({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    refresh: mockRefresh,
    ...overrides
  })
}

const inProgressGame = create(GameSchema, {
  id: 10,
  name: 'Hades',
  playtime: 1200,
  completionRate: '60.00',
  imageUrl: 'https://example.com/hades.jpg'
})

const notStartedGame = create(GameSchema, {
  id: 11,
  name: 'Celeste',
  playtime: 0,
  completionRate: '0.00'
})

const doneGame = create(GameSchema, {
  id: 12,
  name: 'Portal',
  completionRate: '100.00',
  favourite: true
})

function mockSteam(opts: { completed?: (typeof doneGame)[] } = {}) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogSteam.mockReturnValue({
    data: create(GetSteamResponseSchema, {
      steam: create(SteamResponseSchema, {
        inProgress: [inProgressGame],
        notStarted: [notStartedGame],
        completed: opts.completed ?? [],
        totalBacklog: 2,
        currentRate: '1/week'
      })
    }),
    error: undefined,
    isLoading: false
  })
}

describe('GamesLibrary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockShelf = null
    mockWishlistError = undefined
    mockNoWishlistData = false
    mockWishlist = [{ id: 5, name: 'Hollow Knight', imageUrl: '' }]
    mockRefreshState()
  })

  it('shows only the selected shelf', () => {
    mockSteam()
    mockShelf = 'in-progress'
    render(<GamesLibrary />)
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
    expect(screen.queryByText('Hollow Knight')).not.toBeInTheDocument()
  })

  it('shows the wishlist on its shelf', () => {
    mockSteam()
    mockShelf = 'wishlist'
    render(<GamesLibrary />)
    expect(screen.getByText('Hollow Knight')).toBeInTheDocument()
    expect(screen.queryByText('Hades')).not.toBeInTheDocument()
    expect(screen.queryByText('No games on this shelf.')).not.toBeInTheDocument()
  })

  it('shows an empty message for an empty shelf', () => {
    mockSteam()
    mockShelf = 'done'
    render(<GamesLibrary />)
    expect(screen.getByText('No games on this shelf.')).toBeInTheDocument()
  })

  it('labels every shelf tab with its count', () => {
    mockSteam()
    render(<GamesLibrary />)
    const names = screen.getAllByRole('tab').map((t) => t.textContent)
    expect(names).toEqual([
      'All2',
      'Wishlist1',
      'Not started1',
      'In progress1',
      'Done0',
      'Favourites0'
    ])
  })

  it('shows the not started, done and favourites shelves', () => {
    mockSteam({ completed: [doneGame] })
    mockShelf = 'not-started'
    const { unmount } = render(<GamesLibrary />)
    expect(screen.getByText('Celeste')).toBeInTheDocument()
    expect(screen.queryByText('Hades')).not.toBeInTheDocument()
    unmount()
    mockShelf = 'done'
    const done = render(<GamesLibrary />)
    expect(screen.getByText('Portal')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Completed (1)' })).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
    done.unmount()
    mockShelf = 'favourites'
    render(<GamesLibrary />)
    expect(screen.getByText('Portal')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Favourites (1)' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Completed (1)' })).not.toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'All3' })).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
  })

  it('falls back to all for an unknown shelf', () => {
    mockSteam()
    mockShelf = 'bogus'
    render(<GamesLibrary />)
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.getByText('Celeste')).toBeInTheDocument()
    expect(screen.getByText('Hollow Knight')).toBeInTheDocument()
  })

  it('drops the shelf param when returning to all', () => {
    mockSteam()
    mockShelf = 'done'
    render(<GamesLibrary />)
    fireEvent.click(screen.getByRole('tab', { name: 'All2' }))
    expect(mockReplace).toHaveBeenCalledWith('/games/library')
  })

  it('explains an empty shelf versus an empty search', () => {
    mockSteam()
    mockShelf = 'in-progress'
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), { target: { value: 'zzz' } })
    expect(screen.getByText('No games match your search.')).toBeInTheDocument()
    expect(screen.queryByText('No games on this shelf.')).not.toBeInTheDocument()
  })

  it('does not show a no-match message for a blank search', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), { target: { value: '  ' } })
    expect(screen.queryByText(/No games/)).not.toBeInTheDocument()
    expect(screen.getByText('Hades')).toBeInTheDocument()
  })

  it('filters the wishlist by search, ignoring case and padding', () => {
    mockSteam()
    mockShelf = 'wishlist'
    mockWishlist = [
      { id: 5, name: 'Hollow Knight', imageUrl: '' },
      { id: 6, name: 'Balatro', imageUrl: '' }
    ]
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: ' HOLLOW ' }
    })
    expect(screen.getByText('Hollow Knight')).toBeInTheDocument()
    expect(screen.queryByText('Balatro')).not.toBeInTheDocument()
  })

  it('says so when the wishlist shelf is empty, and hides it on all', () => {
    mockSteam()
    mockWishlist = []
    const { unmount } = render(<GamesLibrary />)
    expect(screen.queryByText('Nothing on your wishlist.')).not.toBeInTheDocument()
    unmount()
    mockShelf = 'wishlist'
    render(<GamesLibrary />)
    expect(screen.getByText('Nothing on your wishlist.')).toBeInTheDocument()
    expect(screen.queryByText('No games on this shelf.')).not.toBeInTheDocument()
  })

  it('treats a blank search on an empty shelf as no search', () => {
    mockSteam()
    mockShelf = 'done'
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), { target: { value: '  ' } })
    expect(screen.getByText('No games on this shelf.')).toBeInTheDocument()
  })

  it('keeps tab counts stable while searching', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), { target: { value: 'zz' } })
    expect(screen.getByRole('tab', { name: 'All2' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Wishlist1' })).toBeInTheDocument()
  })

  it('does not report no matches when only the wishlist matches', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), { target: { value: 'hollow' } })
    expect(screen.queryByText(/No games/)).not.toBeInTheDocument()
    expect(screen.getByText('Hollow Knight')).toBeInTheDocument()
  })

  it('keeps Steam library status off the wishlist shelf', () => {
    mockSteam()
    mockShelf = 'wishlist'
    render(<GamesLibrary />)
    expect(screen.queryByText(/Total backlog/)).not.toBeInTheDocument()
  })

  it('shows a wishlist error instead of the list', () => {
    mockSteam()
    mockShelf = 'wishlist'
    mockWishlistError = new Error('boom')
    render(<GamesLibrary />)
    expect(screen.getByText('Failed to load Steam wishlist.')).toBeInTheDocument()
    expect(screen.queryByText('Hollow Knight')).not.toBeInTheDocument()
  })

  it('shows every group with games and no empty message when only some are empty', () => {
    mockSteam({ completed: [doneGame] })
    mockWishlist = []
    render(<GamesLibrary />)
    expect(screen.queryByText(/No games/)).not.toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'All3' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Done1' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Favourites1' })).toBeInTheDocument()
  })

  it('shows no empty message when only some groups are empty', () => {
    mockSteam()
    mockWishlist = []
    render(<GamesLibrary />)
    expect(screen.queryByText(/No games/)).not.toBeInTheDocument()
  })

  it('renders no wishlist section while the wishlist is loading', () => {
    mockSteam()
    mockNoWishlistData = true
    render(<GamesLibrary />)
    expect(screen.queryByRole('heading', { name: /^Wishlist \(/ })).not.toBeInTheDocument()
  })

  it('shows zero counts while Steam is loading', () => {
    // @ts-expect-error -- partial SWRResponse
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<GamesLibrary />)
    expect(screen.getByRole('tab', { name: 'All0' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Favourites0' })).toBeInTheDocument()
  })

  it('hides Steam loading and error states on the wishlist shelf only', () => {
    // @ts-expect-error -- partial SWRResponse
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: new Error('x'), isLoading: true })
    mockShelf = 'wishlist'
    const { unmount } = render(<GamesLibrary />)
    expect(screen.queryByText('Loading Steam library…')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load Steam data.')).not.toBeInTheDocument()
    unmount()
    mockShelf = null
    render(<GamesLibrary />)
    expect(screen.getByText('Loading Steam library…')).toBeInTheDocument()
    expect(screen.getByText('Failed to load Steam data.')).toBeInTheDocument()
  })

  it('shows a wishlist error on the all shelf but not on others', () => {
    mockSteam()
    mockWishlistError = new Error('boom')
    const { unmount } = render(<GamesLibrary />)
    expect(screen.getByText('Failed to load Steam wishlist.')).toBeInTheDocument()
    unmount()
    mockShelf = 'done'
    render(<GamesLibrary />)
    expect(screen.queryByText('Failed to load Steam wishlist.')).not.toBeInTheDocument()
  })

  it('writes the chosen shelf to the URL', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.click(screen.getByRole('tab', { name: 'Not started1' }))
    expect(mockReplace).toHaveBeenCalledWith('/games/library?shelf=not-started')
  })

  it('renders the grouped backlog', () => {
    mockSteam()
    render(<GamesLibrary />)
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.getByText('Celeste')).toBeInTheDocument()
    expect(screen.getByText('In Progress (1)')).toBeInTheDocument()
    expect(screen.getByText('Not Started (1)')).toBeInTheDocument()
    expect(screen.getByText('Completion: 60.00%')).toBeInTheDocument()
    expect(screen.getByText('Completion: 0.00%')).toBeInTheDocument()
  })

  it('filters games by the search input', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'hades' }
    })
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search excludes every game', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'nonexistent' }
    })
    expect(screen.getByText('No games match your search.')).toBeInTheDocument()
  })

  it('triggers a refresh when the button is clicked', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(mockRefresh).toHaveBeenCalled()
  })

  it('disables the button and shows a refreshing label while syncing', () => {
    mockSteam()
    mockRefreshState({ isRefreshing: true })
    render(<GamesLibrary />)
    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
  })

  it('shows the last refresh time when not refreshing', () => {
    mockSteam()
    mockRefreshState({ lastRefresh: new Date('2026-01-02T03:04:05Z') })
    render(<GamesLibrary />)
    expect(screen.getByText(/^Last:/)).toBeInTheDocument()
  })

  it('re-fetches steam data when a sync completes', () => {
    mockSteam()
    render(<GamesLibrary />)
    const onSynced = mockUseSteamRefresh.mock.calls[0]![0]
    onSynced?.()
    expect(mutate).toHaveBeenCalledWith('/games')
  })

  it('renders the game icon with a locked square aspect ratio', () => {
    mockSteam()
    render(<GamesLibrary />)
    const img = screen.getByAltText('Hades')
    expect(img).toHaveClass('h-8', 'w-8', 'object-cover')
  })

  it('links a game card to its detail page', () => {
    mockSteam()
    render(<GamesLibrary />)
    expect(screen.getByText('Hades').closest('a')).toHaveAttribute('href', '/games/10')
  })

  it('lets a game be marked favourite from the card view', async () => {
    mockSteam()
    render(<GamesLibrary />)
    const toggle = screen.getAllByRole('button', { name: 'Add to favourites' })[0]!
    fireEvent.click(toggle)
    await waitFor(() => expect(mockSetGameFavourite).toHaveBeenCalledWith(10, true))
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<GamesLibrary />)
    expect(screen.getByText('Loading Steam library…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    render(<GamesLibrary />)
    expect(screen.getByText('Failed to load Steam data.')).toBeInTheDocument()
  })
})
