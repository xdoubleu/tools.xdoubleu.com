import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogSteamGame: jest.fn()
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
  completionRate: '85%',
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

  it('renders achievements section when achievements exist', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements/)).toBeInTheDocument()
    expect(screen.getByText('Achievement 1')).toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()
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
    expect(screen.getByText('Completion: 85%')).toBeInTheDocument()
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
    expect(screen.getByText('Achieved')).toBeInTheDocument()
    expect(screen.getByText('Locked')).toBeInTheDocument()
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
