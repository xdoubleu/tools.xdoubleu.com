'use client'

import { useReportWebVitals } from 'next/web-vitals'

type ReportWebVitalsCallback = Parameters<typeof useReportWebVitals>[0]

// Beacons each Web Vitals sample to POST /metrics. Module-level so
// useReportWebVitals doesn't re-report every render.
const reportWebVital: ReportWebVitalsCallback = (metric) => {
  const body = JSON.stringify({ name: metric.name, value: metric.value })

  if (typeof navigator.sendBeacon === 'function') {
    navigator.sendBeacon('/metrics', new Blob([body], { type: 'application/json' }))
    return
  }
  void fetch('/metrics', {
    method: 'POST',
    body,
    headers: { 'Content-Type': 'application/json' },
    keepalive: true
  }).catch(() => {})
}

export function WebVitals(): null {
  useReportWebVitals(reportWebVital)
  return null
}
