'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge, type BadgeProps } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose
} from '@/components/ui/dialog'
import { Select } from '@/components/ui/select'
import {
  SecurityAlertType,
  type GetSecurityAlertsResponse,
  type SecurityAlert
} from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'
import { formatDate } from '@/lib/dates'
import { useDismissSecurityAlert } from '@/hooks/useMonitoring'

const SEVERITY_VARIANT: Record<string, BadgeProps['variant']> = {
  critical: 'danger',
  high: 'danger',
  medium: 'warn',
  low: 'secondary'
}

const ALERT_TYPE_LABEL: Record<SecurityAlertType, string> = {
  [SecurityAlertType.UNSPECIFIED]: 'Unknown',
  [SecurityAlertType.DEPENDABOT]: 'Dependabot',
  [SecurityAlertType.CODE_SCANNING]: 'Code scanning',
  [SecurityAlertType.SECRET_SCANNING]: 'Secret scanning'
}

// DISMISS_REASONS lists the exact "dismissed_reason"/"resolution" values
// GitHub's API accepts, per alert type — matching
// api/internal/github/models.go's dependabotDismissReasons/
// codeScanningDismissReasons/secretScanningDismissReasons.
const DISMISS_REASONS: Record<SecurityAlertType, { value: string; label: string }[]> = {
  [SecurityAlertType.UNSPECIFIED]: [],
  [SecurityAlertType.DEPENDABOT]: [
    { value: 'fix_started', label: 'Fix started' },
    { value: 'inaccurate', label: 'Inaccurate or incorrect' },
    { value: 'no_bandwidth', label: 'No bandwidth to fix' },
    { value: 'not_used', label: 'Vulnerable code is not used' },
    { value: 'tolerable_risk', label: 'Risk is tolerable' }
  ],
  [SecurityAlertType.CODE_SCANNING]: [
    { value: 'false positive', label: 'False positive' },
    { value: "won't fix", label: "Won't fix" },
    { value: 'used in tests', label: 'Used in tests' }
  ],
  [SecurityAlertType.SECRET_SCANNING]: [
    { value: 'false_positive', label: 'False positive' },
    { value: 'wont_fix', label: "Won't fix" },
    { value: 'revoked', label: 'Revoked' },
    { value: 'used_in_tests', label: 'Used in tests' },
    { value: 'pattern_deleted', label: 'Pattern no longer detected' }
  ]
}

function alertSubtitle(alert: SecurityAlert) {
  switch (alert.alertType) {
    case SecurityAlertType.CODE_SCANNING:
      return alert.filePath ? `${alert.filePath}:${alert.line}` : alert.ruleId
    case SecurityAlertType.SECRET_SCANNING:
      return alert.secretType
    default:
      return alert.packageName
  }
}

function DismissAlertDialog({
  alert,
  open,
  onOpenChange
}: {
  alert: SecurityAlert
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const dismissSecurityAlert = useDismissSecurityAlert()
  const reasons = DISMISS_REASONS[alert.alertType] ?? []
  const [reason, setReason] = useState(reasons[0]?.value ?? '')
  const [dismissing, setDismissing] = useState(false)
  const [error, setError] = useState('')

  async function handleDismiss() {
    setDismissing(true)
    setError('')
    try {
      await dismissSecurityAlert(alert.alertType, alert.number, reason)
      onOpenChange(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to dismiss alert.')
    } finally {
      setDismissing(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Dismiss alert #{alert.number.toString()}</DialogTitle>
          <DialogClose aria-label="Close">x</DialogClose>
        </DialogHeader>
        <DialogDescription>
          This resolves the alert on GitHub. Pick the reason that best describes why it no longer
          needs attention.
        </DialogDescription>
        <div className="mt-4 space-y-4">
          <Select
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            aria-label="Dismissal reason"
          >
            {reasons.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </Select>
          {error && <p className="text-sm text-danger">{error}</p>}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={dismissing}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDismiss} disabled={dismissing || !reason}>
              {dismissing ? 'Dismissing…' : 'Dismiss alert'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function AlertRow({ alert }: { alert: SecurityAlert }) {
  const [dialogOpen, setDialogOpen] = useState(false)

  return (
    <li className="rounded-lg border border-border bg-surface p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <a
          href={alert.url}
          target="_blank"
          rel="noopener noreferrer"
          className="break-words font-medium text-fg hover:text-accent"
        >
          <span className="mr-1 font-mono text-xs text-muted">#{alert.number.toString()}</span>
          {alertSubtitle(alert)}
        </a>
        <span className="shrink-0 text-xs text-muted">
          {alert.alertType === SecurityAlertType.DEPENDABOT && alert.ecosystem
            ? `${alert.ecosystem} · `
            : ''}
          {formatDate(alert.createdAt)}
        </span>
      </div>
      {alert.summary && <p className="mt-1 text-xs text-muted">{alert.summary}</p>}
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <Badge variant="secondary">{ALERT_TYPE_LABEL[alert.alertType]}</Badge>
        {alert.severity && (
          <Badge variant={SEVERITY_VARIANT[alert.severity] ?? 'default'}>{alert.severity}</Badge>
        )}
        <Button
          variant="secondary"
          size="sm"
          className="ml-auto"
          onClick={() => setDialogOpen(true)}
        >
          Dismiss
        </Button>
      </div>

      <DismissAlertDialog alert={alert} open={dialogOpen} onOpenChange={setDialogOpen} />
    </li>
  )
}

export default function SecurityAlertsCard({ data }: { data?: GetSecurityAlertsResponse }) {
  const alerts = data?.alerts ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Security alerts</CardTitle>
          <Badge variant="secondary">GitHub</Badge>
        </div>
        <CardDescription>
          {data ? `${formatCount(data.alertCount)} open security alert(s).` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">GitHub is not configured.</p>
        ) : alerts.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No open security alerts.</p>
        ) : (
          <ul className="space-y-2">
            {alerts.map((alert) => (
              <AlertRow key={alert.number.toString()} alert={alert} />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
