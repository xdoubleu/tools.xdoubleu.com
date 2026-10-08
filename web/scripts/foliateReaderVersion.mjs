// Content hash of the copied foliate-js runtime, written by copy-foliate.mjs
// next to the files and read by sw.js/route.ts to name the reader module cache.
// The foliate-js git pin alone can't name the cache: run-time patches (see
// foliatePaginatorPatch.mjs) change the served bytes without moving the pin,
// so a pin-named cache would keep serving the stale pre-patch JS to phones
// that already opened a book. Content-hashing busts the cache on any patch.
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

/**
 * A short content hash of every file under `root`. `readdir` order isn't
 * guaranteed, so sort names first for a stable hash.
 */
export function foliateContentHash(root) {
  const hasher = createHash('sha256')
  const walk = (dir) => {
    const names = readdirSync(dir).sort()
    for (const name of names) {
      const path = join(dir, name)
      hasher.update(name)
      if (statSync(path).isDirectory()) walk(path)
      else hasher.update(readFileSync(path))
      hasher.update('\0')
    }
  }
  walk(root)
  return hasher.digest('hex').slice(0, 16)
}