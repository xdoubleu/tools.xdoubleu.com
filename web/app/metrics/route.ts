import { timingSafeEqual } from 'node:crypto'

import { getObservabilityIngestSecret } from '@/lib/env'
import { metricsRegistry, recordWebVital } from '@/lib/server/metrics'

// GET /metrics is scraped by Prometheus over the internal Docker network (the
// "web" job in infra/prometheus.yml). `web` is the kamal-proxy catch-all, so
// the endpoint is also reachable from the public internet — it is gated on a
// bearer token (OBSERVABILITY_INGEST_SECRET, the same shared secret
// app/logs/route.ts uses; infra/prometheus.yml's `web` job sends it as
// `Authorization: Bearer`). When the secret is unset the gate is skipped
// rather than closed, matching app/logs/route.ts and keeping a missing-secret
// misconfiguration from silently zeroing every `web_*` metric (issue #1555).
//
// POST /metrics is the same-origin browser Web Vitals beacon
// (app/_components/web-vitals.tsx) and stays unauthenticated by design.
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
