import SeriesBooksClient from '@/components/books/SeriesBooksClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'

export default async function SeriesPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params
  const decoded = decodeURIComponent(name)
  const client = await createServerClient(LibraryService)
  const series = await fetchOrNull(() => client.getSeries({ name: decoded }))
  return (
    <SWRFallback fallback={{}} keyed={series ? [[swrKeys.bookSeries(decoded), series]] : []}>
      <SeriesBooksClient name={decoded} />
    </SWRFallback>
  )
}
