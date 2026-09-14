import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetAutomatedActionsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import AutomatedActionsCard from '@/components/monitoring/AutomatedActionsCard'

beforeEach(() => {
  jest.useFakeTimers().setSystemTime(new Date('2026-01-01T12:00:00Z'))
})

afterEach(() => {
  jest.useRealTimers()
})

describe('AutomatedActionsCard', () => {
  it('shows a loading state without data', () => {
    render(<AutomatedActionsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an empty state with no runs', () => {
    const data = create(GetAutomatedActionsResponseSchema, { actions: [] })
    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('No automated action runs yet.')).toBeInTheDocument()
  })

  it('renders a finished, successful run with duration and a PR link', () => {
    const data = create(GetAutomatedActionsResponseSchema, {
      actions: [
        {
          id: 1n,
          firedAt: '2026-01-01T10:00:00Z',
          triggerSource: 'schedule',
          routineName: 'dependabot-triage',
          finishedAt: '2026-01-01T10:05:00Z',
          outcome: 'succeeded',
          prUrl: 'https://github.com/xdoubleu/tools.xdoubleu.com/pull/999',
          error: ''
        }
      ]
    })

    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('dependabot-triage')).toBeInTheDocument()
    expect(screen.getByText('schedule')).toBeInTheDocument()
    expect(screen.getByText('Succeeded')).toBeInTheDocument()
    expect(screen.getByText('5.0 min')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'View PR' })
    expect(link).toHaveAttribute('href', 'https://github.com/xdoubleu/tools.xdoubleu.com/pull/999')
  })

  it('renders a run with no PR as a dash', () => {
    const data = create(GetAutomatedActionsResponseSchema, {
      actions: [
        {
          id: 2n,
          firedAt: '2026-01-01T10:00:00Z',
          triggerSource: 'manual',
          routineName: 'sentry-triage',
          finishedAt: '2026-01-01T10:02:00Z',
          outcome: 'no_action_needed',
          prUrl: '',
          error: ''
        }
      ]
    })

    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('No action needed')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'View PR' })).not.toBeInTheDocument()
  })

  it('renders a failed run', () => {
    const data = create(GetAutomatedActionsResponseSchema, {
      actions: [
        {
          id: 3n,
          firedAt: '2026-01-01T10:00:00Z',
          triggerSource: 'api',
          routineName: 'issue-triage',
          finishedAt: '2026-01-01T10:01:00Z',
          outcome: 'failed',
          prUrl: '',
          error: 'boom'
        }
      ]
    })

    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('Failed')).toBeInTheDocument()
  })

  it('flags a still-open run under the threshold as running, not overdue', () => {
    const data = create(GetAutomatedActionsResponseSchema, {
      actions: [
        {
          id: 4n,
          firedAt: '2026-01-01T11:50:00Z', // 10 minutes ago
          triggerSource: 'schedule',
          routineName: 'monitoring-sweep',
          finishedAt: '',
          outcome: '',
          prUrl: '',
          error: ''
        }
      ]
    })

    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('Running…')).toBeInTheDocument()
    expect(screen.getByText('Still running…')).toBeInTheDocument()
    expect(screen.queryByText('Overdue')).not.toBeInTheDocument()
  })

  it('flags a still-open run past the stale threshold as overdue', () => {
    const data = create(GetAutomatedActionsResponseSchema, {
      actions: [
        {
          id: 5n,
          firedAt: '2026-01-01T10:00:00Z', // 2 hours ago
          triggerSource: 'schedule',
          routineName: 'ready-issues-sweep',
          finishedAt: '',
          outcome: '',
          prUrl: '',
          error: ''
        }
      ]
    })

    render(<AutomatedActionsCard data={data} />)
    expect(screen.getByText('Overdue')).toBeInTheDocument()
    expect(screen.getByText('Still running (overdue)')).toBeInTheDocument()
  })
})
