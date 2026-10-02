import { serviceWorkerScript } from '@/lib/offline/serviceWorker'

export const dynamic = 'force-dynamic'

// OFFLINE_DISABLED=1 serves a worker that clears its caches and unregisters.
export function GET() {
  return new Response(serviceWorkerScript(process.env.OFFLINE_DISABLED !== '1'), {
    headers: {
      'Content-Type': 'application/javascript; charset=utf-8',
      'Cache-Control': 'no-cache, no-store, must-revalidate'
    }
  })
}
