'use client'

import { useReportWebVitals } from 'next/web-vitals'

type ReportWebVitalsCallback = Parameters<typeof useReportWebVitals>[0]

// reportWebVital beacons one Core Web Vitals sample to POST /metrics, where
// it lands in the web_vitals_seconds histogram Grafana's FrontendP95High
// alert evaluates (issue #1528). The reference is module-level so
// useReportWebVitals doesn't re-report on every render.
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
