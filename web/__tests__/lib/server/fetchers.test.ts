import { ConnectError, Code } from '@connectrpc/connect'
import { fetchOrNull } from '@/lib/server/fetchers'

describe('fetchOrNull', () => {
  it('returns the fetched data on success', async () => {
    await expect(fetchOrNull(async () => ({ ok: true }))).resolves.toEqual({
      ok: true
    })
  })

  it('returns null when the API rejects with Unauthenticated', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new ConnectError('not signed in', Code.Unauthenticated)
      })
    ).resolves.toBeNull()
  })

  it('returns null when the API rejects with PermissionDenied', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new ConnectError('nope', Code.PermissionDenied)
      })
    ).resolves.toBeNull()
  })

  it('returns null on transient Connect failures', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new ConnectError('api down', Code.Unavailable)
      })
    ).resolves.toBeNull()
  })

  it('rethrows non-Connect errors', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new TypeError('bug in caller')
      })
    ).rejects.toThrow('bug in caller')
  })
})
