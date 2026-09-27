import { NextResponse } from 'next/server'
import { getApiUrl, getObservabilityIngestSecret } from '@/lib/env'
import { clientIP, createRateLimiter } from '@/lib/rateLimit'

// Proxies client log batches to the api's ingest, adding
// OBSERVABILITY_INGEST_SECRET server-side. Not under /api: kamal-proxy
// routes that to the Go service. The endpoint is public, so it relays only
// small same-origin batches at a bounded per-IP rate; the api validates the
// entries and tags them source=web-client.
export const dynamic = 'force-dynamic'

const MAX_BODY_BYTES = 64 * 1024

const limiter = createRateLimiter(60, 60_000)

function crossSite(request: Request): boolean {
  const site = request.headers.get('sec-fetch-site')
  return site !== null && site !== 'same-origin'
}

export async function POST(request: Request): Promise<NextResponse> {
  const secret = getObservabilityIngestSecret()
  if (!secret) return new NextResponse(null, { status: 204 })

  if (crossSite(request)) return new NextResponse(null, { status: 403 })
  if (limiter.limited(clientIP(request))) return new NextResponse(null, { status: 429 })
  if (Number(request.headers.get('content-length') ?? 0) > MAX_BODY_BYTES) {
    return new NextResponse(null, { status: 413 })
  }

  const body = await request.text()
  if (body.length > MAX_BODY_BYTES) return new NextResponse(null, { status: 413 })

  try {
    await fetch(`${getApiUrl()}/api/observability/logs?source=web-client`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Observability-Ingest-Secret': secret
      },
      body
    })
  } catch {
    // Best-effort.
  }

  return new NextResponse(null, { status: 204 })
}
