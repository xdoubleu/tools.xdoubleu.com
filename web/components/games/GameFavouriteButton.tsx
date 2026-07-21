'use client'

import { useState, type MouseEvent } from 'react'
import { mutate } from 'swr'
import { useSetGameFavourite } from '@/hooks/useGames'
import type { Game } from '@/lib/gen/games/v1/games_pb'
import { cn } from '@/lib/cn'
import { swrKeys } from '@/lib/swrKeys'

interface GameFavouriteButtonProps {
  game: Game
  className?: string
}

// Heart toggle mirroring BookFavouriteButton: optimistic flip with rollback
// on error, then revalidate the game and library caches.
export default function GameFavouriteButton({ game, className }: GameFavouriteButtonProps) {
  const [favourite, setFavourite] = useState(game.favourite)
  const setGameFavourite = useSetGameFavourite()

  const handleClick = async (e: MouseEvent) => {
    // Stop the click from bubbling to a wrapping card <Link> (card view).
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
    <button
      type="button"
      onClick={handleClick}
      aria-label={favourite ? 'Remove from favourites' : 'Add to favourites'}
      aria-pressed={favourite}
      className={cn(
        'text-sm leading-none transition-colors',
        'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent',
        favourite ? 'text-amber-500' : 'text-border hover:text-amber-400',
        className
      )}
    >
      ♥
    </button>
  )
}
