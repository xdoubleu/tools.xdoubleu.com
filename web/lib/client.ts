import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
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

export function createServiceClient<T extends DescService>(service: T) {
  return createClient(service, transport)
}
