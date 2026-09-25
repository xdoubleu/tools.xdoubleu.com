import { Histogram, Registry, collectDefaultMetrics } from 'prom-client'

// Prometheus registry for /metrics (browser Web Vitals for Grafana alerts),
// kept on globalThis so dev-mode reloads don't double-register.
type MetricsState = {
  registry: Registry
  webVitals: Histogram<'metric'>
}

const globalForMetrics = globalThis as typeof globalThis & {
  __webMetrics__?: MetricsState
}

function createState(): MetricsState {
  const registry = new Registry()
  registry.setDefaultLabels({ job: 'web' })
  collectDefaultMetrics({ register: registry })

  const webVitals = new Histogram({
    name: 'web_vitals_seconds',
    help: 'Core Web Vitals reported by the browser, in seconds.',
    labelNames: ['metric'],
    // Values are stored in seconds; buckets mirror the api request histogram.
    buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
    registers: [registry]
  })

  return { registry, webVitals }
}

const state = (globalForMetrics.__webMetrics__ ??= createState())

export const metricsRegistry = state.registry

// Bounds label cardinality; unknown names are dropped.
const KNOWN_WEB_VITALS = new Set(['CLS', 'FCP', 'INP', 'LCP', 'TTFB', 'FID'])

// recordWebVital observes one browser metric (ms, or unitless CLS) on the
// seconds-scaled histogram.
export function recordWebVital(name: string, valueMs: number): boolean {
  if (!KNOWN_WEB_VITALS.has(name) || !Number.isFinite(valueMs) || valueMs < 0) {
    return false
  }
  state.webVitals.labels(name).observe(valueMs / 1000)
  return true
}
