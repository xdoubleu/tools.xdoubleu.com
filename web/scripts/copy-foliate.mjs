// Copies foliate-js's runtime files into public/foliate-js/ (gitignored) so the
// browser loads them unbundled: its pdf.js resolves the worker, cmaps and
// standard fonts relative to its own import.meta.url. Runs before dev and build.
import { cpSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const source = dirname(require.resolve('foliate-js/view.js'))
const target = join(dirname(fileURLToPath(import.meta.url)), '..', 'public', 'foliate-js')

rmSync(target, { recursive: true, force: true })
cpSync(source, target, {
  recursive: true,
  filter: (path) => {
    const rel = path.slice(source.length)
    return !/^\/(\.|tests|rollup|node_modules|package|eslint)|\.(map|html|md)$/.test(rel)
  }
})
// pdf.js's modern build calls Map#getOrInsertComputed (Chrome 145, Safari 26.2);
// polyfill it in both the page and the worker for older browsers.
const polyfill = `for (const C of [Map, WeakMap]) {
  if (!C.prototype.getOrInsertComputed) {
    Object.defineProperty(C.prototype, 'getOrInsertComputed', {
      configurable: true,
      writable: true,
      value(key, compute) {
        if (!this.has(key)) this.set(key, compute(key))
        return this.get(key)
      }
    })
  }
}
`
for (const file of ['pdf.mjs', 'pdf.worker.mjs']) {
  const path = join(target, 'vendor', 'pdfjs', file)
  writeFileSync(path, polyfill + readFileSync(path, 'utf8'))
}
console.log(`foliate-js copied to ${target}`)
