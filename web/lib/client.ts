import { createConnectTransport } from '@connectrpc/connect-web'
import { createPromiseClient } from '@connectrpc/connect'
import type { ServiceType } from '@bufbuild/protobuf'

export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL ?? '',
  fetch: (input, init) =>
    fetch(input, {
      ...init,
      credentials: 'include'
    })
})

export function createServiceClient<T extends ServiceType>(service: T) {
  return createPromiseClient(service, transport)
}
