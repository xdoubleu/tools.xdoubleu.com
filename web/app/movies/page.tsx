import MoviesClient from '@/components/movies/MoviesClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import { DEFAULT_BACKLOG_FILTER } from '@/lib/movies/format'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { isSharedList, LIST_PARAM, sharedField } from '@/lib/listScope'

export default async function MoviesPage({
  searchParams
}: {
  searchParams: Promise<{ [LIST_PARAM]?: string }>
}) {
  const shared = isSharedList((await searchParams)[LIST_PARAM])
  const { status, mediaType, sort, onMyServices } = DEFAULT_BACKLOG_FILTER
  const client = await createServerClient(MoviesService)
  const backlog = await fetchOrNull(() =>
    client.listBacklog({
      status,
      mediaType,
      sort,
      onMyServices,
      limit: DEFAULT_PAGE_SIZE,
      ...sharedField(shared)
    })
  )

  return (
    <SWRFallback
      fallback={{}}
      keyed={
        backlog
          ? [[swrKeys.moviesBacklog(status, mediaType, sort, onMyServices, shared), backlog]]
          : []
      }
    >
      <MoviesClient />
    </SWRFallback>
  )
}
