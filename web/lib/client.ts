import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient, type Client } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from './env'

export const transport = createConnectTransport({
  baseUrl: getApiUrl(),
  // Binary avoids base64 inflation of bytes fields (a 75 MB upload would
  // exceed the server cap).
  useBinaryFormat: true,
  fetch: (input, init) =>
    fetch(input, {
      ...init,
      credentials: 'include'
    })
})

// One client per service, reused for the page's lifetime.
const clients = new Map<DescService, Client<DescService>>()

export function createServiceClient<T extends DescService>(service: T): Client<T> {
  let client = clients.get(service)
  if (!client) {
    client = createClient(service, transport)
    clients.set(service, client)
  }
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the map stores each client under its own service descriptor, so the entry for T is always a Client<T>
  return client as Client<T>
}
