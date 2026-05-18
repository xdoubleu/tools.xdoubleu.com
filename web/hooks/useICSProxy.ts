import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ICSProxyService } from '@/lib/gen/icsproxy/v1/proxy_connect'
import type {
  ListConfigsResponse,
  PreviewEventsResponse,
} from '@/lib/gen/icsproxy/v1/proxy_pb'

export function useICSFeeds() {
  const client = createServiceClient(ICSProxyService)
  return useSWR<ListConfigsResponse, Error>('/icsproxy', () =>
    client.listConfigs({})
  )
}

export function useICSPreview(sourceUrl: string) {
  const client = createServiceClient(ICSProxyService)
  return useSWR<PreviewEventsResponse, Error>(
    sourceUrl ? `/icsproxy/preview?url=${encodeURIComponent(sourceUrl)}` : null,
    () => client.previewEvents({ sourceUrl })
  )
}
