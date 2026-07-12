'use client'

import Link from 'next/link'
import Image from 'next/image'
import { useSteamDistribution } from '@/hooks/useGames'
import type { Game, GetSteamDistributionResponse } from '@/lib/gen/games/v1/games_pb'
import { cn } from '@/lib/cn'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

function GameCard({ game, bucket, label }: { game: Game; bucket: number; label: string }) {
  const href = `/games/${game.id}?bucket=${bucket}&label=${encodeURIComponent(label)}`
  return (
    <Link href={href} className={cn(interactiveCardClass, 'relative flex gap-3 p-4')}>
      <CardLinkStatus />
      {game.imageUrl && (
        <Image
          src={game.imageUrl}
          alt={game.name}
          width={32}
          height={32}
          className="h-8 w-8 rounded-lg object-cover shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold">{game.name}</h3>
        <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
        <p className="text-sm text-muted">Completion: {game.completionRate}%</p>
      </div>
    </Link>
  )
}

export default function SteamDistributionClient({
  bucket,
  initialData
}: {
  bucket: string
  initialData?: GetSteamDistributionResponse
}) {
  const bucketNum = Number(bucket)
  const { data, error, isLoading } = useSteamDistribution(bucketNum, initialData)

  const label = data?.data?.label ?? `${bucket}% range`
  const games = data?.data?.games ?? []

  return (
    <PageContainer className="p-6">
      <Breadcrumb items={[{ label: 'Games', href: '/games' }, { label }]} />

      {isLoading && <p className="mt-6 text-muted">Loading…</p>}
      {error && <p className="mt-6 text-danger">Failed to load distribution.</p>}

      {!isLoading && (
        <>
          <h1 className="text-3xl font-bold mt-4 mb-6">{label}</h1>
          {games.length === 0 && <p className="text-muted">No games in this range.</p>}
          {games.length > 0 && (
            <div className="grid sm:grid-cols-2 gap-3">
              {games.map((g) => (
                <GameCard key={g.id} game={g} bucket={bucketNum} label={label} />
              ))}
            </div>
          )}
        </>
      )}
    </PageContainer>
  )
}
