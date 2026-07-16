import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GameSchema } from '@/lib/gen/games/v1/games_pb'

const mockSetGameFavourite = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useGames', () => ({
  useSetGameFavourite: () => mockSetGameFavourite
}))

import GameFavouriteButton from '@/components/games/GameFavouriteButton'

function makeGame(favourite = false) {
  return create(GameSchema, {
    id: 42,
    name: 'Test Game',
    favourite
  })
}

describe('GameFavouriteButton', () => {
  beforeEach(() => {
    mockSetGameFavourite.mockReset()
    mockMutate.mockReset()
    mockSetGameFavourite.mockResolvedValue({})
  })

  it('is not pressed when not a favourite', () => {
    render(<GameFavouriteButton game={makeGame()} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
  })

  it('is pressed when already a favourite', () => {
    render(<GameFavouriteButton game={makeGame(true)} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'true')
  })

  it('calls SetGameFavourite with true when toggled on', async () => {
    render(<GameFavouriteButton game={makeGame()} />)

    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(mockSetGameFavourite).toHaveBeenCalledWith(42, true)
    })
    expect(mockMutate).toHaveBeenCalledWith('/games/42')
    expect(mockMutate).toHaveBeenCalledWith('/games')
  })

  it('calls SetGameFavourite with false when toggled off', async () => {
    render(<GameFavouriteButton game={makeGame(true)} />)

    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(mockSetGameFavourite).toHaveBeenCalledWith(42, false)
    })
  })

  it('reverts to previous state on error', async () => {
    mockSetGameFavourite.mockRejectedValue(new Error('fail'))
    render(<GameFavouriteButton game={makeGame()} />)

    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
    })
  })
})
