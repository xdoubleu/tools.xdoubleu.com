/**
 * @jest-environment node
 */
import { POST } from '@/app/logs/route'

jest.mock('@/lib/env', () => ({
  getApiUrl: jest.fn(),
  getObservabilityIngestSecret: jest.fn()
}))

import { getApiUrl, getObservabilityIngestSecret } from '@/lib/env'

const mockedGetApiUrl = jest.mocked(getApiUrl)
const mockedGetSecret = jest.mocked(getObservabilityIngestSecret)
const mockFetch = jest.fn()

describe('POST /logs', () => {
  beforeEach(() => {
    jest.resetAllMocks()
    global.fetch = mockFetch
  })

  it('returns 204 without calling the api when no secret is configured', async () => {
    mockedGetSecret.mockReturnValue('')

    const request = new Request('http://localhost/logs', {
      method: 'POST',
      body: '{"entries":[]}'
    })
    const response = await POST(request)

    expect(response.status).toBe(204)
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('forwards the batch to the api with the secret header', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    mockedGetApiUrl.mockReturnValue('http://api.internal')
    mockFetch.mockResolvedValue(new Response(null))

    const body = '{"entries":[{"level":"info","message":"hi"}]}'
    const request = new Request('http://localhost/logs', {
      method: 'POST',
      body
    })
    const response = await POST(request)

    expect(response.status).toBe(204)
    expect(mockFetch).toHaveBeenCalledWith(
      'http://api.internal/api/observability/logs',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'X-Observability-Ingest-Secret': 'shhh'
        }),
        body
      })
    )
  })

  it('still returns 204 when the upstream fetch fails', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    mockedGetApiUrl.mockReturnValue('http://api.internal')
    mockFetch.mockRejectedValue(new Error('network down'))

    const request = new Request('http://localhost/logs', {
      method: 'POST',
      body: '{"entries":[]}'
    })
    const response = await POST(request)

    expect(response.status).toBe(204)
  })
})
