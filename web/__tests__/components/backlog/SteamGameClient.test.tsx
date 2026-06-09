import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogSteamGame: jest.fn()
}))

let mockSearchParams = new URLSearchParams()
jest.mock('next/navigation', () => ({
  useSearchParams: () => mockSearchParams
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('next/image', () => {
  return function MockImage({
    src,
    alt,
    ...props
  }: {
    src: string
    alt: string
    [key: string]: unknown
  }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

import SteamGameClient from '@/app/backlog/games/[id]/SteamGameClient'
import { useBacklogSteamGame } from '@/hooks/useBacklog'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  AchievementSchema,
  GetSteamGameResponseSchema,
  SteamGameResponseSchema
} from '@/lib/gen/backlog/v1/games_pb'

const mockGame = create(GameSchema, {
  id: 1,
  name: 'The Witcher 3',
  playtime: 3600,
  completionRate: '85.00',
  isDelisted: false
})

const mockAchievements = [
  create(AchievementSchema, {
    name: 'ach1',
    displayName: 'Achievement 1',
    description: 'First achievement',
    achieved: true,
    globalPercent: 50.5,
    iconUrl: 'http://example.com/ach1.png'
  }),
  create(AchievementSchema, {
    name: 'ach2',
    displayName: 'Achievement 2',
    description: 'Second achievement',
    achieved: false,
    globalPercent: 25.3,
    iconUrl: 'http://example.com/ach2.png'
  })
]

beforeEach(() => {
  jest.clearAllMocks()
  mockSearchParams = new URLSearchParams()
})

const mockSteamGameResponse = (
  game: ReturnType<typeof create<typeof GameSchema>>,
  achievements: ReturnType<typeof create<typeof AchievementSchema>>[]
) =>
  create(GetSteamGameResponseSchema, {
    data: create(SteamGameResponseSchema, { game, achievements })
  })

describe('SteamGameClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Loading game...')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to fetch')
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Failed to load game.')).toBeInTheDocument()
  })

  it('renders game name when game is loaded', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByRole('heading', { name: 'The Witcher 3' })).toBeInTheDocument()
  })

  it('renders achievements section with unlocked achievements by default', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements/)).toBeInTheDocument()
    // Achievement 1 is achieved (completed) so it is hidden by default.
    expect(screen.queryByText('Achievement 1')).not.toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()
  })

  it('renders the achievement icon with a locked square aspect ratio', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByAltText('Achievement 2')).toHaveClass('h-12', 'w-12', 'object-cover')
  })

  it('orders achievements by descending global percent', () => {
    const achievements = [
      create(AchievementSchema, {
        name: 'low',
        displayName: 'Low percent',
        achieved: false,
        globalPercent: 10.5
      }),
      create(AchievementSchema, {
        name: 'high',
        displayName: 'High percent',
        achieved: false,
        globalPercent: 80.2
      }),
      create(AchievementSchema, {
        name: 'mid',
        displayName: 'Mid percent',
        achieved: false,
        globalPercent: 40.1
      })
    ]
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, achievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const headings = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(headings).toEqual(['High percent', 'Mid percent', 'Low percent'])
  })

  it('toggles completed achievements visibility', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const toggle = screen.getByRole('button', { name: 'Show completed' })
    fireEvent.click(toggle)
    expect(screen.getByText('Achievement 1')).toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Hide completed' }))
    expect(screen.queryByText('Achievement 1')).not.toBeInTheDocument()
  })

  it('shows all-completed message when every achievement is unlocked', () => {
    const allDone = [
      create(AchievementSchema, {
        name: 'done',
        displayName: 'Done',
        achieved: true,
        globalPercent: 30
      })
    ]
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, allDone),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('All achievements completed.')).toBeInTheDocument()
  })

  it('shows no achievements message when list is empty', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('No achievements for this game.')).toBeInTheDocument()
  })

  it('calls useBacklogSteamGame with numeric id converted from string', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="456" />)
    expect(useBacklogSteamGame).toHaveBeenCalledWith(456)
  })

  it('renders games link', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const backlogLink = screen.getByText(/Games/).closest('a')
    expect(backlogLink).toHaveAttribute('href', '/backlog/games')
  })

  it('does not render a distribution breadcrumb without a bucket param', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText('80-89%')).not.toBeInTheDocument()
  })

  it('renders a distribution breadcrumb linking back to the bucket overview', () => {
    mockSearchParams = new URLSearchParams({ bucket: '8', label: '80-89%' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const bucketLink = screen.getByText('80-89%').closest('a')
    expect(bucketLink).toHaveAttribute('href', '/backlog/games/distribution/8')
  })

  it('falls back to a range label when only the bucket param is present', () => {
    mockSearchParams = new URLSearchParams({ bucket: '3' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const bucketLink = screen.getByText('3% range').closest('a')
    expect(bucketLink).toHaveAttribute('href', '/backlog/games/distribution/3')
  })

  it('displays playtime in hours', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/60\s+hrs played/)).toBeInTheDocument()
  })

  it('displays completion rate', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Completion: 85.00%')).toBeInTheDocument()
  })

  it('displays achieved count in achievements header', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements \(1\/2\)/)).toBeInTheDocument()
  })

  it('renders achievement cards with correct statuses', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    // Locked is visible by default; Achieved appears once completed are shown.
    expect(screen.getByText('Locked')).toBeInTheDocument()
    expect(screen.queryByText('Achieved')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Show completed' }))
    expect(screen.getByText('Achieved')).toBeInTheDocument()
  })

  it('shows delisted badge when game is delisted', () => {
    const delistedGame = create(GameSchema, { ...mockGame, isDelisted: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(delistedGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Delisted')).toBeInTheDocument()
  })

  it('does not show delisted badge when game is not delisted', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText('Delisted')).not.toBeInTheDocument()
  })
})
