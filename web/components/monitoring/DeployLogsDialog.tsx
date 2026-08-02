'use client'

import * as Sentry from '@sentry/nextjs'
import { useEffect, useState } from 'react'
import { useDeployLogs } from '@/hooks/useMonitoring'
import type { DeployComponentLog } from '@/lib/gen/observability/v1/observability_pb'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

interface DeployLogsDialogProps {
  deploymentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

function LogSection({ log, deploymentId }: { log: DeployComponentLog; deploymentId: string }) {
  // Runtime logs come from the deployment actually serving traffic, which is
  // a different one whenever the requested deploy failed — say which.
  const foreign = log.deploymentId !== '' && log.deploymentId !== deploymentId

  return (
    <div className="rounded-xl border border-border bg-surface p-3">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium text-fg">{log.component}</span>
        <Badge variant="secondary">{log.logType}</Badge>
        {foreign && <Badge variant="secondary">deployment {log.deploymentId}</Badge>}
        {log.truncated && <Badge variant="warn">truncated</Badge>}
      </div>
      <div className="max-h-64 overflow-auto rounded-lg bg-card p-2">
        <pre className="whitespace-pre-wrap break-all font-mono text-xs text-fg">{log.content}</pre>
      </div>
    </div>
  )
}

export default function DeployLogsDialog({
  deploymentId,
  open,
  onOpenChange
}: DeployLogsDialogProps) {
  const fetchDeployLogs = useDeployLogs()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [logs, setLogs] = useState<DeployComponentLog[]>([])

  useEffect(() => {
    if (!open) return
    setError('')
    setLoading(true)
    fetchDeployLogs(deploymentId)
      .then((resp) => setLogs(resp.logs))
      .catch((err) => {
        Sentry.captureException(err)
        setError('Failed to load deploy logs.')
      })
      .finally(() => setLoading(false))
  }, [open, deploymentId, fetchDeployLogs])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="right" className="max-w-2xl">
        <DialogHeader>
          <div>
            <DialogTitle>Deploy logs</DialogTitle>
            <DialogDescription>
              Build and deploy output for this deployment, plus the runtime output of the deployment
              currently serving traffic.
            </DialogDescription>
          </div>
          <DialogClose aria-label="Close">x</DialogClose>
        </DialogHeader>

        {loading ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : error ? (
          <p className="py-8 text-center text-sm text-danger">{error}</p>
        ) : logs.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No logs available yet.</p>
        ) : (
          <div className="space-y-3">
            {logs.map((log) => (
              <LogSection
                key={`${log.deploymentId}-${log.component}-${log.logType}`}
                log={log}
                deploymentId={deploymentId}
              />
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
