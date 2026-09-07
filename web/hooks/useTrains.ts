import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TrainService } from '@/lib/gen/trains/v1/trains_pb'
import type {
  GetFeedInfoResponse,
  GetJourneyDetailResponse,
  SearchJourneysResponse,
  Station
} from '@/lib/gen/trains/v1/trains_pb'

/** The CC BY attribution string's date, driven from the imported feed's own version. */
export function useTrainsFeedInfo() {
  const client = createServiceClient(TrainService)
  return useSWR<GetFeedInfoResponse, Error>(swrKeys.trainsFeedInfo, () => client.getFeedInfo({}))
}

/** Type-ahead station search backing the origin/destination pickers. */
export function useStationSearch(query: string) {
  const client = createServiceClient(TrainService)
  const trimmed = query.trim()
  const { data, isLoading } = useSWR<{ stations: Station[] }, Error>(
    swrKeys.trainsStations(trimmed),
    () => client.searchStations({ query: trimmed }),
    { keepPreviousData: true }
  )
  return { stations: data?.stations ?? [], isLoading }
}

/**
 * Journey search over the requested (origin, destination, time, arriveBy)
 * criteria. Returns null keys — SWR does not fetch — until both stations are
 * chosen, so no request fires while the form is still being filled in.
 */
export function useJourneySearch(
  originStopId: string,
  destinationStopId: string,
  time: string,
  arriveBy: boolean
) {
  const client = createServiceClient(TrainService)
  const ready = originStopId !== '' && destinationStopId !== ''
  return useSWR<SearchJourneysResponse, Error>(
    ready ? swrKeys.trainsJourneys(originStopId, destinationStopId, time, arriveBy) : null,
    () => client.searchJourneys({ originStopId, destinationStopId, time, arriveBy })
  )
}

/**
 * The live journey detail page's data source (issue #1394): an initial fetch
 * via GetJourneyDetail, revalidated on demand by useJourneyLive's
 * reconnect-and-refetch path rather than on any fixed interval — the
 * websocket push is what keeps it current while the page stays open.
 */
export function useJourneyDetail(journeyId: string) {
  const client = createServiceClient(TrainService)
  return useSWR<GetJourneyDetailResponse, Error>(
    journeyId ? swrKeys.trainsJourneyDetail(journeyId) : null,
    () => client.getJourneyDetail({ journeyId }),
    { revalidateOnFocus: false }
  )
}
