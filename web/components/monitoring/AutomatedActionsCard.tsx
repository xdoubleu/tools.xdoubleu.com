'use client'

import { SectionCard } from '@/components/ui/section-card'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell
} from '@/components/ui/table'
import type { GetAutomatedActionsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDuration, isAutomatedActionStale } from '@/lib/observability'
import { formatDateTime } from '@/lib/dates'

function outcomeBadge(outcome: string, stale: boolean) {
  if (!outcome) {
    return stale ? (
      <Badge variant="warn">Overdue</Badge>
    ) : (
      <Badge variant="secondary">Running…</Badge>
    )
  }
  if (outcome === 'succeeded') return <Badge variant="success">Succeeded</Badge>
  if (outcome === 'failed') return <Badge variant="danger">Failed</Badge>
  if (outcome === 'no_action_needed') return <Badge variant="secondary">No action needed</Badge>
  return <Badge variant="secondary">{outcome}</Badge>
}

function durationCell(firedAt: string, finishedAt: string, stale: boolean): string {
  if (!finishedAt) return stale ? 'Still running (overdue)' : 'Still running…'
  const fired = new Date(firedAt)
  const finished = new Date(finishedAt)
  if (Number.isNaN(fired.getTime()) || Number.isNaN(finished.getTime())) return '—'
  return formatDuration(finished.getTime() - fired.getTime())
}

export default function AutomatedActionsCard({ data }: { data?: GetAutomatedActionsResponse }) {
  const actions = data?.actions ?? []

  return (
    <SectionCard
      title="Automated actions"
      description={
        data ? 'Self-healing routines that ran outside this app over the last 30 days.' : 'Loading…'
      }
    >
      {data && actions.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted">No automated action runs yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Fired at</TableHead>
              <TableHead>Trigger</TableHead>
              <TableHead>Routine</TableHead>
              <TableHead>Duration</TableHead>
              <TableHead>Outcome</TableHead>
              <TableHead>PR</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {actions.map((action) => {
              const stale = isAutomatedActionStale(action.firedAt, action.finishedAt)
              return (
                <TableRow key={action.id.toString()} data-selected={stale || undefined}>
                  <TableCell className="whitespace-nowrap text-sm">
                    {formatDateTime(action.firedAt)}
                  </TableCell>
                  <TableCell className="text-sm text-muted">{action.triggerSource}</TableCell>
                  <TableCell className="text-sm font-medium">{action.routineName}</TableCell>
                  <TableCell
                    className={`text-sm whitespace-nowrap ${stale ? 'text-warn font-medium' : 'text-muted'}`}
                  >
                    {durationCell(action.firedAt, action.finishedAt, stale)}
                  </TableCell>
                  <TableCell title={action.error || undefined}>
                    {outcomeBadge(action.outcome, stale)}
                  </TableCell>
                  <TableCell className="text-sm">
                    {action.prUrl ? (
                      <a
                        href={action.prUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="-m-2 inline-block p-2 text-accent hover:underline"
                      >
                        View PR
                      </a>
                    ) : (
                      <span className="text-muted">—</span>
                    )}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </SectionCard>
  )
}
