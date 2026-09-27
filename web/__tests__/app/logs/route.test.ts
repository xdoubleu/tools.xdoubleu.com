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
      'http://api.internal/api/observability/logs?source=web-client',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'X-Observability-Ingest-Secret': 'shhh'
        }),
        body
      })
    )
  })

  it('refuses cross-site requests', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    const request = new Request('http://localhost/logs', {
      method: 'POST',
      headers: { 'sec-fetch-site': 'cross-site' },
      body: '{"entries":[]}'
    })
    const response = await POST(request)

    expect(response.status).toBe(403)
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('refuses oversized bodies', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    const request = new Request('http://localhost/logs', {
      method: 'POST',
      headers: { 'x-forwarded-for': '203.0.113.9' },
      body: 'x'.repeat(64 * 1024 + 1)
    })
    const response = await POST(request)

    expect(response.status).toBe(413)
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('refuses a declared oversized body without reading it', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    const request = new Request('http://localhost/logs', {
      method: 'POST',
      headers: { 'content-length': String(1024 * 1024), 'x-forwarded-for': '203.0.113.8' },
      body: '{}'
    })
    const response = await POST(request)

    expect(response.status).toBe(413)
  })

  it('rate-limits a single client', async () => {
    mockedGetSecret.mockReturnValue('shhh')
    mockedGetApiUrl.mockReturnValue('http://api.internal')
    mockFetch.mockResolvedValue(new Response(null))

    const send = () =>
      POST(
        new Request('http://localhost/logs', {
          method: 'POST',
          headers: { 'x-forwarded-for': 'spoofed, 198.51.100.7' },
          body: '{"entries":[]}'
        })
      )
    for (let i = 0; i < 30; i++) expect((await send()).status).toBe(204)
    expect((await send()).status).toBe(429)
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
