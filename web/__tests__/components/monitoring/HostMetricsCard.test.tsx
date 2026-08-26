import { hostMetricTone } from '@/components/monitoring/HostMetricsCard'

describe('hostMetricTone', () => {
  it('flags danger at or above the danger threshold', () => {
    expect(hostMetricTone(90)).toBe('danger')
    expect(hostMetricTone(97)).toBe('danger')
  })

  it('flags warn at or above the warn threshold but below danger', () => {
    expect(hostMetricTone(75)).toBe('warn')
    expect(hostMetricTone(89.9)).toBe('warn')
  })

  it('flags default below the warn threshold', () => {
    expect(hostMetricTone(0)).toBe('default')
    expect(hostMetricTone(74.9)).toBe('default')
  })
})
