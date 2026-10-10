import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { LibraryService, type GetSeriesResponse } from '@/lib/gen/books/v1/library_pb'

/** A series' volumes: the user's books plus the Hardcover volumes they lack. */
export function useBookSeries(name: string) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetSeriesResponse, Error>(name ? swrKeys.bookSeries(name) : null, () =>
    client.getSeries({ name })
  )
}
