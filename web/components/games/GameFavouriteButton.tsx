'use client'

import { useState, type MouseEvent } from 'react'
import { mutate } from 'swr'
import { useSetGameFavourite } from '@/hooks/useGames'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { ToggleIconButton } from '@/components/ui/toggle-icon-button'
import { swrKeys } from '@/lib/swrKeys'

interface GameFavouriteButtonProps {
  game: Game
  className?: string
}

// Optimistic flip with rollback, then revalidate game and library caches.
export default function GameFavouriteButton({ game, className }: GameFavouriteButtonProps) {
  const [favourite, setFavourite] = useState(game.favourite)
  const setGameFavourite = useSetGameFavourite()

  const handleClick = async (e: MouseEvent) => {
    // Don't trigger a wrapping card <Link>.
    e.preventDefault()
    e.stopPropagation()

    const newFavourite = !favourite
    const prev = favourite
    setFavourite(newFavourite)
    try {
      await setGameFavourite(game.id, newFavourite)
      mutate(swrKeys.game(game.id))
      mutate(swrKeys.games)
    } catch {
      setFavourite(prev)
    }
  }

  return (
    <ToggleIconButton
      active={favourite}
      onToggle={handleClick}
      label="Add to favourites"
      activeLabel="Remove from favourites"
      className={className}
    >
      ♥
    </ToggleIconButton>
  )
}
