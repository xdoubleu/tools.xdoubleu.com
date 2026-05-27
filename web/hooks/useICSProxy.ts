import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import { ICSProxyService, SaveConfigRequestSchema } from '@/lib/gen/icsproxy/v1/proxy_pb'
import type {
  ListConfigsResponse,
  PreviewEventsResponse,
  GetConfigResponse
} from '@/lib/gen/icsproxy/v1/proxy_pb'

export type SaveConfigInput = MessageInitShape<typeof SaveConfigRequestSchema>

export function useICSFeeds() {
  const client = createServiceClient(ICSProxyService)
  return useSWR<ListConfigsResponse, Error>('/icsproxy', () => client.listConfigs({}))
}

export function useICSPreview(sourceUrl: string) {
  const client = createServiceClient(ICSProxyService)
  return useSWR<PreviewEventsResponse, Error>(
    sourceUrl ? `/icsproxy/preview?url=${encodeURIComponent(sourceUrl)}` : null,
    () => client.previewEvents({ sourceUrl })
  )
}

export function useICSConfig(token: string) {
  const client = createServiceClient(ICSProxyService)
  return useSWR<GetConfigResponse, Error>(token ? `/icsproxy/${token}` : null, () =>
    client.getConfig({ token })
  )
}

export function useSaveConfig() {
  const client = createServiceClient(ICSProxyService)
  return (req: SaveConfigInput) => client.saveConfig(req)
}

export function useDeleteConfig() {
  const client = createServiceClient(ICSProxyService)
  return (token: string) => client.deleteConfig({ token })
}
