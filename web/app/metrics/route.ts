import { metricsRegistry, recordWebVital } from '@/lib/server/metrics'

// Prometheus scrapes GET /metrics over the internal Docker network (the
// "web" job in infra/prometheus.yml, issue #1528). POST /metrics is the
// same-origin browser beacon from the WebVitals component.
export const runtime = 'nodejs'
export const dynamic = 'force-dynamic'

export async function GET(): Promise<Response> {
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
