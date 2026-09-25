import { timingSafeEqual } from 'node:crypto'

import { getObservabilityIngestSecret } from '@/lib/env'
import { metricsRegistry, recordWebVital } from '@/lib/server/metrics'

// GET: Prometheus scrape. Public via the kamal-proxy catch-all, so gated on
// OBSERVABILITY_INGEST_SECRET as a bearer token; skipped when unset so a
// missing secret doesn't zero every metric.
// POST: the unauthenticated same-origin Web Vitals beacon.
export const runtime = 'nodejs'
export const dynamic = 'force-dynamic'

function bearerAuthorized(request: Request, secret: string): boolean {
  const header = request.headers.get('authorization') ?? ''
  const prefix = 'Bearer '
  if (!header.startsWith(prefix)) return false
  const provided = Buffer.from(header.slice(prefix.length))
  const expected = Buffer.from(secret)
  return provided.length === expected.length && timingSafeEqual(provided, expected)
}

export async function GET(request: Request): Promise<Response> {
  const secret = getObservabilityIngestSecret()
  if (secret && !bearerAuthorized(request, secret)) {
    return new Response('unauthorized', { status: 401 })
  }

  return new Response(await metricsRegistry.metrics(), {
    headers: { 'Content-Type': metricsRegistry.contentType }
  })
}

export async function POST(request: Request): Promise<Response> {
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return new Response('invalid json', { status: 400 })
  }

  const { name, value } = (body ?? {}) as { name?: unknown; value?: unknown }
  if (typeof name !== 'string' || typeof value !== 'number') {
    return new Response('expected {name: string, value: number}', { status: 400 })
  }

  const recorded = recordWebVital(name, value)
  return new Response(null, { status: recorded ? 204 : 202 })
}
