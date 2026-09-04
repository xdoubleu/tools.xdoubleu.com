import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetSecurityAlertsResponseSchema,
  SecurityAlertType
} from '@/lib/gen/observability/v1/observability_pb'
import SecurityAlertsCard from '@/components/monitoring/SecurityAlertsCard'

const mockDismissSecurityAlert = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useDismissSecurityAlert: () => mockDismissSecurityAlert
}))

const dependabotAlert = {
  number: 83n,
  packageName: 'otel',
  ecosystem: 'go',
  severity: 'medium',
  summary: 'unbounded body read',
  url: 'https://gh/alerts/83',
  createdAt: '2026-08-19T16:34:44Z',
  alertType: SecurityAlertType.DEPENDABOT,
  ruleId: '',
  filePath: '',
  line: 0,
  secretType: ''
}

const secretAlert = {
  number: 7n,
  packageName: '',
  ecosystem: '',
  severity: '',
  summary: '',
  url: 'https://gh/alerts/7',
  createdAt: '2026-08-21T08:00:00Z',
  alertType: SecurityAlertType.SECRET_SCANNING,
  ruleId: '',
  filePath: '',
  line: 0,
  secretType: 'AWS Access Key'
}

const codeScanningAlertWithFile = {
  number: 12n,
  packageName: '',
  ecosystem: '',
  severity: 'unmapped-severity',
  summary: '',
  url: 'https://gh/alerts/12',
  createdAt: '2026-08-22T08:00:00Z',
  alertType: SecurityAlertType.CODE_SCANNING,
  ruleId: 'js/unused-var',
  filePath: 'src/index.ts',
  line: 42,
  secretType: ''
}

const codeScanningAlertWithoutFile = {
  ...codeScanningAlertWithFile,
  number: 13n,
  filePath: '',
  severity: ''
}

describe('SecurityAlertsCard', () => {
  beforeEach(() => {
    mockDismissSecurityAlert.mockReset()
    mockDismissSecurityAlert.mockResolvedValue(undefined)
  })

  it('shows a loading state without data', () => {
    render(<SecurityAlertsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows a not-configured message', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: false,
      alerts: [],
      alertCount: 0
    })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('GitHub is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state with no alerts', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alerts: [],
      alertCount: 0
    })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('No open security alerts.')).toBeInTheDocument()
  })

  it('opens a confirmation dialog with type-specific reasons and dismisses on confirm', async () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [dependabotAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #83')).toBeInTheDocument()

    // Dependabot-specific reasons are offered.
    expect(screen.getByRole('option', { name: 'No bandwidth to fix' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'Revoked' })).not.toBeInTheDocument()

    fireEvent.change(screen.getByRole('combobox', { name: 'Dismissal reason' }), {
      target: { value: 'no_bandwidth' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss alert' }))

    await waitFor(() =>
      expect(mockDismissSecurityAlert).toHaveBeenCalledWith(
        SecurityAlertType.DEPENDABOT,
        83n,
        'no_bandwidth'
      )
    )
    await waitFor(() => expect(screen.queryByText('Dismiss alert #83')).not.toBeInTheDocument())
  })

  it('blocks confirming until a reason is picked', async () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [dependabotAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #83')).toBeInTheDocument()

    // No reason chosen yet, so the destructive action must not be reachable.
    fireEvent.change(screen.getByRole('combobox', { name: 'Dismissal reason' }), {
      target: { value: '' }
    })
    expect(screen.getByRole('button', { name: 'Dismiss alert' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss alert' }))
    expect(mockDismissSecurityAlert).not.toHaveBeenCalled()
  })

  it('offers secret-scanning-specific reasons for a secret-scanning alert', async () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [secretAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #7')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Revoked' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: 'No bandwidth to fix' })).not.toBeInTheDocument()
  })

  it('shows an error and keeps the dialog open when dismissing fails', async () => {
    mockDismissSecurityAlert.mockRejectedValue(new Error('boom'))
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [dependabotAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #83')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss alert' }))

    expect(await screen.findByText('boom')).toBeInTheDocument()
    expect(screen.getByText('Dismiss alert #83')).toBeInTheDocument()
  })

  it('shows the file path and line, plus an unmapped severity, for a code-scanning alert with a file', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [codeScanningAlertWithFile]
    })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('src/index.ts:42')).toBeInTheDocument()
    expect(screen.getByText('unmapped-severity')).toBeInTheDocument()
  })

  it('falls back to the rule ID for a code-scanning alert with no file path', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [codeScanningAlertWithoutFile]
    })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('js/unused-var')).toBeInTheDocument()
  })

  it('shows a generic error message when the rejection is not an Error', async () => {
    mockDismissSecurityAlert.mockRejectedValue('boom')
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [dependabotAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #83')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss alert' }))

    expect(await screen.findByText('Failed to dismiss alert.')).toBeInTheDocument()
  })

  it('closes the dialog without dismissing on cancel', async () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [dependabotAlert]
    })
    render(<SecurityAlertsCard data={data} />)

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(await screen.findByText('Dismiss alert #83')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByText('Dismiss alert #83')).not.toBeInTheDocument())
    expect(mockDismissSecurityAlert).not.toHaveBeenCalled()
  })
})
