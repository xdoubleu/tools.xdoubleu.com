import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient, type Client } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from './env'

export const transport = createConnectTransport({
  baseUrl: getApiUrl(),
  // Binary format avoids base64 inflation for bytes fields (e.g. ebook uploads).
  // Without this, a 75 MB file becomes ~100 MB on the wire and trips the server cap.
  useBinaryFormat: true,
  fetch: (input, init) =>
    fetch(input, {
      ...init,
      credentials: 'include'
    })
})

// Clients are stateless wrappers around the shared transport, so one instance
// per service descriptor is reused for the lifetime of the page.
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
