import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { foliateContentHash } from '../../../scripts/foliateReaderVersion.mjs'

const tmp = () => mkdtempSync(join(tmpdir(), 'foliate-'))

describe('foliateContentHash', () => {
  it('is stable for identical content', () => {
    const root = tmp()
    writeFileSync(join(root, 'paginator.js'), 'const a = 1\n')
    expect(foliateContentHash(root)).toBe(foliateContentHash(root))
  })

  it('is independent of readdir order', () => {
    const one = tmp()
    writeFileSync(join(one, 'a.js'), 'a')
    writeFileSync(join(one, 'b.js'), 'b')
    const expected = foliateContentHash(one)
    const other = tmp()
    writeFileSync(join(other, 'b.js'), 'b')
    writeFileSync(join(other, 'a.js'), 'a')
    expect(foliateContentHash(other)).toBe(expected)
  })

  it('changes when a file changes', () => {
    const root = tmp()
    writeFileSync(join(root, 'paginator.js'), 'old')
    const before = foliateContentHash(root)
    writeFileSync(join(root, 'paginator.js'), 'new')
    expect(foliateContentHash(root)).not.toBe(before)
  })

  it('hashes a nested tree', () => {
    const root = tmp()
    mkdirSync(join(root, 'vendor', 'pdfjs'), { recursive: true })
    writeFileSync(join(root, 'a.js'), 'a')
    writeFileSync(join(root, 'vendor', 'pdfjs', 'pdf.mjs'), 'pdf')
    expect(foliateContentHash(root)).toBe(foliateContentHash(root))
  })
})
