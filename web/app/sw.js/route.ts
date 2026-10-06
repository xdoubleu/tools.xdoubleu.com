import { serviceWorkerScript } from '@/lib/offline/serviceWorker'
import packageJson from '@/package.json'

export const dynamic = 'force-dynamic'

// The pinned foliate-js commit names the reader's module cache.
const READER_VERSION = packageJson.dependencies['foliate-js'].replace(/^.*#/, '')

// OFFLINE_DISABLED=1 serves a worker that clears its caches and unregisters.
export function GET() {
  return new Response(serviceWorkerScript(process.env.OFFLINE_DISABLED !== '1', READER_VERSION), {
    headers: {
      'Content-Type': 'application/javascript; charset=utf-8',
      'Cache-Control': 'no-cache, no-store, must-revalidate'
    }
  })
}
