import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { formatDateTime } from '@/lib/dates'
import { GetAlertStatesResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import AlertStatesCard from '@/components/monitoring/AlertStatesCard'

const BREACHING_SINCE = '2026-08-30T10:00:00Z'

describe('AlertStatesCard', () => {
  it('renders a loading description with no data', () => {
    render(<AlertStatesCard />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders an empty state when there are no rules', () => {
    render(<AlertStatesCard data={create(GetAlertStatesResponseSchema, { states: [] })} />)
    expect(screen.getByText('No threshold alert rules.')).toBeInTheDocument()
  })

  it('formats percent, bytes, and duration units per rule key', () => {
    render(
      <AlertStatesCard
        data={create(GetAlertStatesResponseSchema, {
          states: [
            { ruleKey: 'host_cpu_high', breaching: false, currentValue: 42.25, threshold: 80 },
            {
              ruleKey: 'r2_usage_high',
              breaching: false,
              currentValue: 1024 * 1024 * 1024,
              threshold: 50 * 1024 * 1024 * 1024
            },
            {
              ruleKey: 'ci_duration_high',
              breaching: false,
              currentValue: 60_000,
              threshold: 900_000
            },
            {
              ruleKey: 'slow_transaction_http_high',
              breaching: false,
              currentValue: 1200,
              threshold: 5000
            },
            {
              ruleKey: 'slow_transaction_job_high',
              breaching: false,
              currentValue: 24_000,
              threshold: 60_000
            },
            {
              ruleKey: 'slow_transaction_frontend_high',
              breaching: false,
              currentValue: 1500,
              threshold: 5000
            }
          ]
        })}
      />
    )

    // percent keeps one decimal
    expect(screen.getByText(/42\.3% of 80\.0% threshold/)).toBeInTheDocument()
    // bytes and ms go through the shared observability formatters
    expect(screen.getByText(/1\.0 GB of 50\.0 GB threshold/)).toBeInTheDocument()
    expect(screen.getByText(/1\.0 min of 15\.0 min threshold/)).toBeInTheDocument()
    expect(screen.getByText(/1\.2 s of 5\.0 s threshold/)).toBeInTheDocument()
    expect(screen.getByText(/24\.0 s of 1\.0 min threshold/)).toBeInTheDocument()
    expect(screen.getByText(/1\.5 s of 5\.0 s threshold/)).toBeInTheDocument()
    expect(screen.getByText('Slow HTTP handlers (p95)')).toBeInTheDocument()
    expect(screen.getByText('Slow background jobs (p95)')).toBeInTheDocument()
    expect(screen.getByText('Slow frontend spans (p95)')).toBeInTheDocument()
  })

  it('marks a breaching rule, shows its since timestamp, and counts it in the header', () => {
    render(
      <AlertStatesCard
        data={create(GetAlertStatesResponseSchema, {
          states: [
            { ruleKey: 'host_cpu_high', breaching: false, currentValue: 10, threshold: 80 },
            {
              ruleKey: 'host_disk_high',
              breaching: true,
              currentValue: 91,
              threshold: 85,
              since: BREACHING_SINCE
            }
          ]
        })}
      />
    )

    expect(screen.getByText('1 breaching')).toBeInTheDocument()
    expect(screen.getByText('Breaching')).toBeInTheDocument()
    expect(screen.getByText('OK')).toBeInTheDocument()
    expect(screen.getByText(`Since ${formatDateTime(BREACHING_SINCE)}`)).toBeInTheDocument()
  })

  it('sorts breaching rules ahead of healthy ones', () => {
    render(
      <AlertStatesCard
        data={create(GetAlertStatesResponseSchema, {
          states: [
            { ruleKey: 'host_cpu_high', breaching: false, currentValue: 10, threshold: 80 },
            { ruleKey: 'host_memory_high', breaching: true, currentValue: 95, threshold: 85 }
          ]
        })}
      />
    )

    const labels = screen.getAllByText(/Host (CPU|memory) usage/).map((el) => el.textContent)
    expect(labels).toEqual(['Host memory usage', 'Host CPU usage'])
  })

  it('falls back to the raw key and a plain number for an unknown rule', () => {
    render(
      <AlertStatesCard
        data={create(GetAlertStatesResponseSchema, {
          states: [
            { ruleKey: 'future_rule_high', breaching: false, currentValue: 7, threshold: 10 }
          ]
        })}
      />
    )

    expect(screen.getByText('future_rule_high')).toBeInTheDocument()
    expect(screen.getByText(/7 of 10 threshold/)).toBeInTheDocument()
  })

  it('hides the since line for a breaching rule with no since timestamp', () => {
    render(
      <AlertStatesCard
        data={create(GetAlertStatesResponseSchema, {
          states: [{ ruleKey: 'host_cpu_high', breaching: true, currentValue: 95, threshold: 80 }]
        })}
      />
    )

    expect(screen.queryByText(/^Since /)).not.toBeInTheDocument()
  })
})
