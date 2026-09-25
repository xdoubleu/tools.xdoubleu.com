import { NextResponse } from 'next/server'
import { getApiUrl, getObservabilityIngestSecret } from '@/lib/env'

// Proxies client log batches to the api's ingest, adding
// OBSERVABILITY_INGEST_SECRET server-side. Not under /api: kamal-proxy
// routes that to the Go service.
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
    // Best-effort.
  }

  return new NextResponse(null, { status: 204 })
}
