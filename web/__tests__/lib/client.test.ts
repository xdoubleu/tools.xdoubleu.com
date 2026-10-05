/** @jest-environment node */
import {
  createKeepaliveClient,
  createServiceClient,
  keepaliveTransport,
  transport
} from '@/lib/client'
import { AuthService } from '@/lib/gen/auth/v1/auth_pb'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'

describe('createServiceClient', () => {
  it('returns the same client instance for the same service', () => {
    const a = createServiceClient(AuthService)
    const b = createServiceClient(AuthService)
    expect(a).toBe(b)
  })

  it('returns distinct clients for distinct services', () => {
    const a = createServiceClient(AuthService)
    const b = createServiceClient(RecipesService)
    expect(a).not.toBe(b)
  })

  it('builds keepalive clients with the service methods', () => {
    expect(typeof createKeepaliveClient(LibraryService).updateReadingProgress).toBe('function')
  })

  it('exposes the service methods', () => {
    const client = createServiceClient(AuthService)
    expect(typeof client.signIn).toBe('function')
  })
})

describe('transports', () => {
  it('sends keepalive only on the keepalive transport, both with credentials', async () => {
    const fetchMock = jest
      .spyOn(globalThis, 'fetch')
      .mockImplementation(
        async () =>
          new Response(new Uint8Array(), { headers: { 'Content-Type': 'application/proto' } })
      )
    try {
      const method = LibraryService.method.updateReadingProgress
      await transport.unary(method, undefined, undefined, undefined, {})
      await keepaliveTransport.unary(method, undefined, undefined, undefined, {})
      expect(fetchMock.mock.calls.map(([, init]) => [init?.credentials, init?.keepalive])).toEqual([
        ['include', false],
        ['include', true]
      ])
    } finally {
      fetchMock.mockRestore()
    }
  })
})
