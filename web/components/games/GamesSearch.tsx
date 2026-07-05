'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useSteam } from '@/hooks/useGames'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/cn'

const MAX_RESULTS = 5

interface GamesSearchProps {
  className?: string
}

export default function GamesSearch({ className }: GamesSearchProps) {
  const [query, setQuery] = useState('')
  const { data } = useSteam()

  const steam = data?.steam
  const q = query.trim().toLowerCase()

  const results =
    q && steam
      ? [...(steam.notStarted ?? []), ...(steam.inProgress ?? []), ...(steam.completed ?? [])]
          .filter((g) => g.name.toLowerCase().includes(q))
          .slice(0, MAX_RESULTS)
      : []

  return (
    <div className={cn('relative', className)}>
      <Input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search games…"
      />
      {results.length > 0 && (
        <ul className="absolute z-10 mt-1 w-full overflow-y-auto rounded-2xl border border-border bg-card shadow-elevated max-h-48">
          {results.map((game) => (
            <li key={game.id}>
              <Link
                href={`/games/${game.id}`}
                onClick={() => setQuery('')}
                className={cn(
                  'flex w-full items-center gap-2 rounded-lg px-4 py-2 text-left text-sm text-fg',
                  'transition-colors hover:bg-hover',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent'
                )}
              >
                {game.name}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
