'use client'

import { useSteamDistribution } from '@/hooks/useGames'
import type { GetSteamDistributionResponse } from '@/lib/gen/games/v1/games_pb'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { GameCard } from '@/components/games/GameCards'

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
    <PageContainer>
      <Breadcrumb items={[{ label: 'Games', href: '/dashboard/games' }, { label }]} />

      {isLoading && <LoadingState className="mt-6" />}
      {error && <ErrorState what="distribution" className="mt-6" />}

      {!isLoading && (
        <>
          <PageHeader title={label} className="mt-4" />
          {games.length === 0 && <p className="text-muted">No games in this range.</p>}
          {games.length > 0 && (
            <div className="grid sm:grid-cols-2 gap-3">
              {games.map((g) => (
                <GameCard
                  key={g.id}
                  game={g}
                  href={`/games/${g.id}?bucket=${bucketNum}&label=${encodeURIComponent(label)}`}
                  showFavourite
                />
              ))}
            </div>
          )}
        </>
      )}
    </PageContainer>
  )
}
