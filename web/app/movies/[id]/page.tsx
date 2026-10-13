import MovieTitleClient from '@/components/movies/MovieTitleClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { isSharedList, LIST_PARAM, sharedField } from '@/lib/listScope'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'

export default async function MovieTitlePage({
  params,
  searchParams
}: {
  params: Promise<{ id: string }>
  searchParams: Promise<{ [LIST_PARAM]?: string }>
}) {
  const { id } = await params
  const shared = isSharedList((await searchParams)[LIST_PARAM])
  const client = await createServerClient(MoviesService)
  const title = await fetchOrNull(() => client.getTitle({ id, ...sharedField(shared) }))

  return (
    <SWRFallback fallback={title ? { [swrKeys.moviesTitle(id, shared)]: title } : {}}>
      <MovieTitleClient id={id} />
    </SWRFallback>
  )
}
