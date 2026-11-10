import { render } from '@testing-library/react'
import type { Metric } from 'web-vitals'

const handlers: Record<string, (metric: Pick<Metric, 'name' | 'value'>) => void> = {}

vi.mock('web-vitals', () => {
  const register = (name: string) => (cb: (metric: Pick<Metric, 'name' | 'value'>) => void) => {
    handlers[name] = cb
  }
  return {
    onCLS: register('CLS'),
    onFCP: register('FCP'),
    onINP: register('INP'),
    onLCP: register('LCP'),
    onTTFB: register('TTFB')
  }
})

const track = vi.fn()
vi.mock('@/lib/analytics', () => ({ track: (...args: unknown[]) => track(...args) }))

import { WebVitals } from '@/src/WebVitals'

describe('WebVitals', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('observes every core metric', () => {
    render(<WebVitals />)

    expect(Object.keys(handlers).sort()).toEqual(['CLS', 'FCP', 'INP', 'LCP', 'TTFB'])
  })

  it('beacons a reported metric to /web-vitals', async () => {
    const sendBeacon = vi.fn()
    Object.defineProperty(navigator, 'sendBeacon', { value: sendBeacon, configurable: true })

    render(<WebVitals />)
    handlers.TTFB({ name: 'TTFB', value: 120 })

    expect(sendBeacon).toHaveBeenCalledWith('/web-vitals', expect.any(Blob))
    const blob: Blob = sendBeacon.mock.calls[0][1]
    expect(JSON.parse(await blob.text())).toEqual({ name: 'TTFB', value: 120 })
  })

  it('falls back to fetch when sendBeacon is unavailable', () => {
    Object.defineProperty(navigator, 'sendBeacon', { value: undefined, configurable: true })
    const fetchMock = vi.fn(() => Promise.resolve(undefined))
    Object.defineProperty(global, 'fetch', {
      value: fetchMock,
      configurable: true,
      writable: true
    })

    render(<WebVitals />)
    handlers.CLS({ name: 'CLS', value: 0.02 })

    expect(fetchMock).toHaveBeenCalledWith(
      '/web-vitals',
      expect.objectContaining({ method: 'POST', keepalive: true })
    )
  })

  it('sends CLS to PostHog with the route', () => {
    Object.defineProperty(navigator, 'sendBeacon', { value: vi.fn(), configurable: true })

    render(<WebVitals />)
    handlers.CLS({ name: 'CLS', value: 0.3 })
    handlers.TTFB({ name: 'TTFB', value: 120 })

    expect(track).toHaveBeenCalledTimes(1)
    expect(track).toHaveBeenCalledWith('web_vital_cls', { value: 0.3, route: '/' })
  })
})
