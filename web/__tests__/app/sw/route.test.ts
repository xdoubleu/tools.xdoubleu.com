/** @jest-environment node */
jest.mock('node:fs', () => ({ readFileSync: jest.fn() }))
import { readFileSync } from 'node:fs'
import { GET, readerVersionFrom } from '@/app/sw.js/route'

const mockRead = jest.mocked(readFileSync)

// The module-level version read in route.ts runs on import, before each test
// body; default to "missing" so it falls back to a URI-split of the dependency.
mockRead.mockImplementation(() => {
  throw new Error('ENOENT')
})

describe('GET /sw.js', () => {
  beforeEach(() =>
    mockRead.mockImplementation(() => {
      throw new Error('ENOENT')
    })
  )
  afterEach(() => {
    delete process.env.OFFLINE_DISABLED
  })

  it('serves the enabled worker uncached', async () => {
    const res = GET()

    expect(res.headers.get('Content-Type')).toBe('application/javascript; charset=utf-8')
    expect(res.headers.get('Cache-Control')).toBe('no-cache, no-store, must-revalidate')
    expect(await res.text()).toMatch(/\(self, true, "[0-9a-f]+"\);\n$/)
  })

  it('serves the self-unregistering worker when OFFLINE_DISABLED=1', async () => {
    process.env.OFFLINE_DISABLED = '1'

    expect(await GET().text()).toMatch(/\(self, false, "[0-9a-f]+"\);\n$/)
  })
})

describe('readerVersionFrom', () => {
  beforeEach(() =>
    mockRead.mockImplementation(() => {
      throw new Error('ENOENT')
    })
  )

  it('reads the content-hash version file when present', () => {
    mockRead.mockReturnValue('abc123def456')
    expect(readerVersionFrom('/foliate-js/.reader-version', 'pin')).toBe('abc123def456')
  })

  it('falls back to the pinned commit when the file is missing', () => {
    expect(readerVersionFrom('/missing', 'pin-beef')).toBe('pin-beef')
  })
})
