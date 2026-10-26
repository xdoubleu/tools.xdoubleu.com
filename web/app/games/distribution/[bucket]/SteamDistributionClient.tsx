import { useSteamDistribution } from '@/hooks/useGames'
import { Breadcrumb } from '@/components/ui/breadcrumb'
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
      <Breadcrumb items={[{ label: 'Games', href: '/dashboard/games' }, { label }]} />

      {isLoading && <LoadingState className="mt-6" />}
      {error && <ErrorState what="distribution" className="mt-6" />}

      {!isLoading && (
        <>
          <PageHeader title={label} className="mt-4" />
          {games.length === 0 && <EmptyState>No games in this range.</EmptyState>}
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
