import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from './env'

export const transport = createConnectTransport({
  baseUrl: getApiUrl(),
  fetch: (input, init) =>
    fetch(input, {
      ...init,
      credentials: 'include'
    })
})

export function createServiceClient<T extends DescService>(service: T) {
  return createClient(service, transport)
}
