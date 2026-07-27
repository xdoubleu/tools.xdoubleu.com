jest.mock('@sentry/nextjs', () => ({
  captureException: jest.fn()
}))

import { ConnectError, Code } from '@connectrpc/connect'
import * as Sentry from '@sentry/nextjs'
import { fetchOrNull } from '@/lib/server/fetchers'

const mockCaptureException = jest.mocked(Sentry.captureException)

beforeEach(() => {
  jest.clearAllMocks()
})

describe('fetchOrNull', () => {
  it('returns the fetched data on success', async () => {
    await expect(fetchOrNull(async () => ({ ok: true }))).resolves.toEqual({
      ok: true
    })
    expect(mockCaptureException).not.toHaveBeenCalled()
  })

  it('returns null without reporting on Unauthenticated', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new ConnectError('not signed in', Code.Unauthenticated)
      })
    ).resolves.toBeNull()
    expect(mockCaptureException).not.toHaveBeenCalled()
  })

  it('returns null without reporting on PermissionDenied', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new ConnectError('nope', Code.PermissionDenied)
      })
    ).resolves.toBeNull()
    expect(mockCaptureException).not.toHaveBeenCalled()
  })

  it('returns null but reports a hung api (DeadlineExceeded) to Sentry', async () => {
    const err = new ConnectError('timed out', Code.DeadlineExceeded)
    await expect(fetchOrNull(async () => Promise.reject(err))).resolves.toBeNull()
    expect(mockCaptureException).toHaveBeenCalledWith(err)
  })

  it('returns null but reports transient Connect failures to Sentry', async () => {
    const err = new ConnectError('api down', Code.Unavailable)
    await expect(fetchOrNull(async () => Promise.reject(err))).resolves.toBeNull()
    expect(mockCaptureException).toHaveBeenCalledWith(err)
  })

  it('rethrows non-Connect errors', async () => {
    await expect(
      fetchOrNull(async () => {
        throw new TypeError('bug in caller')
      })
    ).rejects.toThrow('bug in caller')
    expect(mockCaptureException).not.toHaveBeenCalled()
  })
})
