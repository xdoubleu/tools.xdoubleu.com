import { useSteamDistribution } from '@/hooks/useGames'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { GameCard } from '@/components/games/GameCards'

export default function SteamDistributionClient({ bucket }: { bucket: string }) {
  const bucketNum = Number(bucket)
  const { data, error, isLoading } = useSteamDistribution(bucketNum)

  const label = data?.data?.label ?? `${bucket}% range`
  const games = data?.data?.games ?? []

  return (
    <PageContainer>
      <PageHeader
        title={label}
        breadcrumb={[{ label: 'Games', href: '/dashboard/games' }, { label }]}
      />

      {isLoading && <LoadingState placement="page" />}
      {error && <ErrorState placement="page" what="distribution" />}

      {!isLoading && (
        <>
          {games.length === 0 && <EmptyState placement="page">No games in this range.</EmptyState>}
          {games.length > 0 && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
