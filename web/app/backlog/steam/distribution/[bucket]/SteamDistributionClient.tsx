'use client'

import Link from 'next/link'
import { useBacklogDistribution } from '@/hooks/useBacklog'
import type { Game } from '@/lib/gen/backlog/v1/games_pb'
import { cn } from '@/lib/cn'
import { interactiveCardClass } from '@/components/ui/card'
import { Breadcrumb } from '@/components/ui/breadcrumb'

function GameCard({ game }: { game: Game }) {
  return (
    <Link href={`/backlog/steam/${game.id}`} className={cn(interactiveCardClass, 'block p-4')}>
      <h3 className="font-semibold">{game.name}</h3>
      <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
      <p className="text-sm text-muted">Completion: {game.completionRate}</p>
    </Link>
  )
}

export default function SteamDistributionClient({ bucket }: { bucket: string }) {
  const bucketNum = Number(bucket)
  const { data, error, isLoading } = useBacklogDistribution(bucketNum)

  const label = data?.data?.label ?? `${bucket}% range`
  const games = data?.data?.games ?? []

  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        items={[
          { label: 'Backlog', href: '/backlog' },
          { label: 'Games', href: '/backlog/steam' },
          { label }
        ]}
      />

      {isLoading && <p className="mt-6 text-muted">Loading...</p>}
      {error && <p className="mt-6 text-danger">Failed to load distribution.</p>}

      {!isLoading && (
        <>
          <h1 className="text-3xl font-bold mt-4 mb-6">{label}</h1>
          {games.length === 0 && <p className="text-muted">No games in this range.</p>}
          {games.length > 0 && (
            <div className="grid sm:grid-cols-2 gap-3">
              {games.map((g) => (
                <GameCard key={g.id} game={g} />
              ))}
            </div>
          )}
        </>
      )}
    </main>
  )
}
