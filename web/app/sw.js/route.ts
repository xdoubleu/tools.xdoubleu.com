import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { serviceWorkerScript } from '@/lib/offline/serviceWorker'
import packageJson from '@/package.json'

export const dynamic = 'force-dynamic'

// The content hash copy-foliate.mjs writes beside the runtime keys the reader
// cache: any patch to the served files (not just a pin bump) busts the SW
// cache. Fall back to the pinned commit when the file isn't built yet.
const READER_VERSION = readerVersionFrom(
  join(process.cwd(), 'public', 'foliate-js', '.reader-version'),
  packageJson.dependencies['foliate-js'].replace(/^.*#/, '')
)

// OFFLINE_DISABLED=1 serves a worker that clears its caches and unregisters.
export function GET() {
  return new Response(serviceWorkerScript(process.env.OFFLINE_DISABLED !== '1', READER_VERSION), {
    headers: {
      'Content-Type': 'application/javascript; charset=utf-8',
      'Cache-Control': 'no-cache, no-store, must-revalidate'
    }
  })
}

export function readerVersionFrom(path: string, fallback: string): string {
  try {
    return readFileSync(path, 'utf8').trim()
  } catch {
    return fallback
  }
}
