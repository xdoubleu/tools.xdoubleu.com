import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { TrainService } from '@/lib/gen/trains/v1/trains_pb'
import type {
  GetFeedInfoResponse,
  GetJourneyDetailResponse,
  ListSavedCommutesResponse,
  SearchJourneysResponse,
  Station
} from '@/lib/gen/trains/v1/trains_pb'

/** Feed version date for the CC BY attribution. */
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

/** Journey search; null key (no fetch) until both stations are chosen. */
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

/** The user's saved commutes, with create/delete/reverse mutators. */
export function useSavedCommutes() {
  const client = createServiceClient(TrainService)
  const swr = useSWR<ListSavedCommutesResponse, Error>(swrKeys.trainsSavedCommutes, () =>
    client.listSavedCommutes({})
  )

  const create = async (label: string, originStopId: string, destinationStopId: string) => {
    await client.createSavedCommute({ label, originStopId, destinationStopId })
    await swr.mutate()
  }

  const remove = async (id: string) => {
    await client.deleteSavedCommute({ id })
    await swr.mutate()
  }

  return { ...swr, create, remove }
}

/**
 * Live journey detail: revalidated by useJourneyLive's reconnect path, not on
 * an interval; pushes keep it current.
 */
export function useJourneyDetail(journeyId: string) {
  const client = createServiceClient(TrainService)
  return useSWR<GetJourneyDetailResponse, Error>(
    journeyId ? swrKeys.trainsJourneyDetail(journeyId) : null,
    () => client.getJourneyDetail({ journeyId }),
    { revalidateOnFocus: false }
  )
}
