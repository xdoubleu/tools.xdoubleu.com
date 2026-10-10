/** @jest-environment node */
import { GET } from '@/app/sw.js/route'

describe('GET /sw.js', () => {
  afterEach(() => {
    delete process.env.OFFLINE_DISABLED
  })

  it('serves the enabled worker uncached', async () => {
    const res = GET()

    expect(res.headers.get('Content-Type')).toBe('application/javascript; charset=utf-8')
    expect(res.headers.get('Cache-Control')).toBe('no-cache, no-store, must-revalidate')
    expect(await res.text()).toMatch(/\(self, true\);\n$/)
  })

  it('serves the self-unregistering worker when OFFLINE_DISABLED=1', async () => {
    process.env.OFFLINE_DISABLED = '1'

    expect(await GET().text()).toMatch(/\(self, false\);\n$/)
  })
})
