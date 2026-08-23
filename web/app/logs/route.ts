import { NextResponse } from 'next/server'
import { getApiUrl, getObservabilityIngestSecret } from '@/lib/env'

// Proxies client-side log batches (lib/logger.ts) to the api's shared-secret
// ingest endpoint, attaching OBSERVABILITY_INGEST_SECRET server-side so the
// browser never sees it. Deliberately off the /api prefix — kamal-proxy
// routes everything under /api to the Go api service, so a route there would
// never actually reach this Next.js handler (see swrKeys.webRelease).
export const dynamic = 'force-dynamic'

export async function POST(request: Request): Promise<NextResponse> {
  const secret = getObservabilityIngestSecret()
  if (!secret) return new NextResponse(null, { status: 204 })

  const body = await request.text()
  try {
    await fetch(`${getApiUrl()}/api/observability/logs`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Observability-Ingest-Secret': secret
      },
      body
    })
  } catch {
    // Best-effort: a dropped log batch isn't worth failing the caller over.
  }

  return new NextResponse(null, { status: 204 })
}
