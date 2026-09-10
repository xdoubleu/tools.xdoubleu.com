/**
 * @jest-environment node
 */
import { GET, POST } from '@/app/metrics/route'

const originalSecret = process.env.OBSERVABILITY_INGEST_SECRET

afterEach(() => {
  process.env.OBSERVABILITY_INGEST_SECRET = originalSecret
})

function get(headers?: Record<string, string>): Request {
  return new Request('http://localhost/metrics', { headers })
}

describe('GET /metrics', () => {
  it('serves Prometheus text exposition with the web_vitals_seconds histogram', async () => {
    delete process.env.OBSERVABILITY_INGEST_SECRET
    const res = await GET(get())
    const body = await res.text()

    expect(res.status).toBe(200)
    expect(res.headers.get('Content-Type')).toContain('text/plain')
    expect(body).toContain('web_vitals_seconds')
    expect(body).toContain('job="web"')
  })

  it('serves unauthenticated when no ingest secret is configured', async () => {
    delete process.env.OBSERVABILITY_INGEST_SECRET
    expect((await GET(get())).status).toBe(200)
  })

  describe('with an ingest secret configured', () => {
    beforeEach(() => {
      process.env.OBSERVABILITY_INGEST_SECRET = 'scrape-secret'
    })

    it('401s a request with no bearer token', async () => {
      expect((await GET(get())).status).toBe(401)
    })

    it('401s a request with the wrong bearer token', async () => {
      expect((await GET(get({ authorization: 'Bearer nope' }))).status).toBe(401)
    })

    it('serves a request with the correct bearer token', async () => {
      const res = await GET(get({ authorization: 'Bearer scrape-secret' }))
      expect(res.status).toBe(200)
      expect(await res.text()).toContain('web_vitals_seconds')
    })
  })
})

describe('POST /metrics', () => {
  function post(payload: unknown): Request {
    return new Request('http://localhost/metrics', {
      method: 'POST',
      body: JSON.stringify(payload),
      headers: { 'Content-Type': 'application/json' }
    })
  }

  it('records a known web vital and reports it on the next scrape', async () => {
    delete process.env.OBSERVABILITY_INGEST_SECRET
    const res = await POST(post({ name: 'LCP', value: 2200 }))
    expect(res.status).toBe(204)

    const body = await (await GET(get())).text()
    expect(body).toMatch(/web_vitals_seconds_bucket\{[^}]*metric="LCP"/)
  })

  it('accepts-but-ignores an unknown metric name', async () => {
    const res = await POST(post({ name: 'BOGUS', value: 1 }))
    expect(res.status).toBe(202)
  })

  it('rejects a malformed payload', async () => {
    expect((await POST(post({ name: 'LCP' }))).status).toBe(400)
  })

  it('rejects invalid json', async () => {
    const bad = new Request('http://localhost/metrics', { method: 'POST', body: '{' })
    expect((await POST(bad)).status).toBe(400)
  })
})
