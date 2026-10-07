import MoviesClient from '@/components/movies/MoviesClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import { DEFAULT_BACKLOG_FILTER } from '@/lib/movies/format'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export default async function MoviesPage() {
  const { status, mediaType, sort } = DEFAULT_BACKLOG_FILTER
  const client = await createServerClient(MoviesService)
  const backlog = await fetchOrNull(() =>
    client.listBacklog({ status, mediaType, sort, limit: DEFAULT_PAGE_SIZE })
  )

  return (
    <SWRFallback
      fallback={{}}
      keyed={backlog ? [[swrKeys.moviesBacklog(status, mediaType, sort), backlog]] : []}
    >
      <MoviesClient />
    </SWRFallback>
  )
}
