'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetDeployStatusResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDateTime } from '@/lib/dates'

function phaseVariant(phase: string): 'success' | 'danger' | 'warn' {
  switch (phase.toUpperCase()) {
    case 'ACTIVE':
      return 'success'
    case 'ERROR':
    case 'CANCELED':
      return 'danger'
    default:
      return 'warn'
  }
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <span className="text-sm text-subtle">{label}</span>
      <span className="break-words text-right text-sm text-fg">{value}</span>
    </div>
  )
}

export default function DeployCard({ data }: { data?: GetDeployStatusResponse }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Deployment</CardTitle>
        <CardDescription>Latest DigitalOcean app deployment.</CardDescription>
      </CardHeader>
      <CardContent>
        {!data ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">DigitalOcean is not configured.</p>
        ) : !data.deploymentId ? (
          <p className="py-8 text-center text-sm text-muted">No deployment recorded.</p>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm text-subtle">Phase</span>
              <Badge variant={phaseVariant(data.phase)}>{data.phase || 'UNKNOWN'}</Badge>
            </div>
            {data.cause && <Field label="Cause" value={data.cause} />}
            <Field label="Created" value={formatDateTime(data.createdAt)} />
            <Field label="Updated" value={formatDateTime(data.updatedAt)} />
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-sm text-subtle">Deployment</span>
              <span className="break-all text-right font-mono text-xs text-muted">
                {data.deploymentId}
              </span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
