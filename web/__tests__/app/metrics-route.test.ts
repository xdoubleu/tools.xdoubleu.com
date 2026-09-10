/**
 * @jest-environment node
 */
import { GET, POST } from '@/app/metrics/route'

describe('GET /metrics', () => {
  it('serves Prometheus text exposition with the web_vitals_seconds histogram', async () => {
    const res = await GET()
    const body = await res.text()

    expect(res.headers.get('Content-Type')).toContain('text/plain')
    expect(body).toContain('web_vitals_seconds')
    expect(body).toContain('job="web"')
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
    const res = await POST(post({ name: 'LCP', value: 2200 }))
    expect(res.status).toBe(204)

    const body = await (await GET()).text()
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
