/**
 * @jest-environment node
 *
 * Run in the Node.js environment (not jsdom) so that File, File.arrayBuffer,
 * and crypto.subtle are all available natively (Node.js 18+). This avoids the
 * need for jsdom polyfills for those APIs.
 */
import { sha256Hex } from '@/lib/backlog/checksum'

describe('sha256Hex', () => {
  it('returns lowercase hex SHA-256 for a known input', async () => {
    // echo -n "hello" | sha256sum → 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
    const file = new File(['hello'], 'test.txt', { type: 'text/plain' })
    const hash = await sha256Hex(file)
    expect(hash).toBe('2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824')
  })

  it('returns 64-character hex string', async () => {
    const file = new File(['arbitrary content'], 'a.epub', { type: 'application/epub+zip' })
    const hash = await sha256Hex(file)
    expect(hash).toHaveLength(64)
    expect(hash).toMatch(/^[0-9a-f]+$/)
  })

  it('returns different hashes for different content', async () => {
    const f1 = new File(['content-a'], 'a.epub', { type: 'application/epub+zip' })
    const f2 = new File(['content-b'], 'b.epub', { type: 'application/epub+zip' })
    const [h1, h2] = await Promise.all([sha256Hex(f1), sha256Hex(f2)])
    expect(h1).not.toBe(h2)
  })

  it('returns the same hash for identical content regardless of filename', async () => {
    const f1 = new File(['same bytes'], 'one.epub', { type: 'application/epub+zip' })
    const f2 = new File(['same bytes'], 'two.epub', { type: 'application/epub+zip' })
    const [h1, h2] = await Promise.all([sha256Hex(f1), sha256Hex(f2)])
    expect(h1).toBe(h2)
  })
})
