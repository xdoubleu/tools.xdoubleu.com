import MovieTitleClient from '@/components/movies/MovieTitleClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'

export default async function MovieTitlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(MoviesService)
  const title = await fetchOrNull(() => client.getTitle({ id }))

  return (
    <SWRFallback fallback={title ? { [swrKeys.moviesTitle(id)]: title } : {}}>
      <MovieTitleClient id={id} />
    </SWRFallback>
  )
}
