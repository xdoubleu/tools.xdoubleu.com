import { render } from '@testing-library/react'

let reportCallback: ((metric: { name: string; value: number }) => void) | undefined

jest.mock('next/web-vitals', () => ({
  useReportWebVitals: (cb: (metric: { name: string; value: number }) => void) => {
    reportCallback = cb
  }
}))

import { WebVitals } from '@/app/_components/web-vitals'

describe('WebVitals', () => {
  afterEach(() => {
    jest.restoreAllMocks()
    reportCallback = undefined
  })

  it('beacons a reported metric to /metrics', () => {
    const sendBeacon = jest.fn()
    Object.defineProperty(navigator, 'sendBeacon', { value: sendBeacon, configurable: true })

    render(<WebVitals />)
    reportCallback?.({ name: 'TTFB', value: 120 })

    expect(sendBeacon).toHaveBeenCalledWith('/metrics', expect.any(Blob))
  })

  it('falls back to fetch when sendBeacon is unavailable', () => {
    Object.defineProperty(navigator, 'sendBeacon', { value: undefined, configurable: true })
    const fetchMock = jest.fn(() => Promise.resolve(undefined))
    Object.defineProperty(global, 'fetch', {
      value: fetchMock,
      configurable: true,
      writable: true
    })

    render(<WebVitals />)
    reportCallback?.({ name: 'CLS', value: 0.02 })

    expect(fetchMock).toHaveBeenCalledWith('/metrics', expect.objectContaining({ method: 'POST' }))
  })
})
