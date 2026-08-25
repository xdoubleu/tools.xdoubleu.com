'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetWorkflowRunsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDuration } from '@/lib/observability'
import { formatDate } from '@/lib/dates'

function eventLabel(event: string): string {
  return event === 'push' ? 'main' : 'PR'
}

function conclusionVariant(conclusion: string): 'success' | 'danger' | 'secondary' {
  if (conclusion === 'success') return 'success'
  if (conclusion === '') return 'secondary'
  return 'danger'
}

export default function WorkflowRunsCard({
  data,
  title = 'CI run duration',
  description
}: {
  data?: GetWorkflowRunsResponse
  title?: string
  description?: string
}) {
  const runs = data?.runs ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {data ? (description ?? 'Recent PR and main GitHub Actions run times.') : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">GitHub is not configured.</p>
        ) : runs.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No workflow runs.</p>
        ) : (
          <ul className="space-y-2">
            {runs.map((run) => (
              <li
                key={run.id.toString()}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={run.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="break-words font-medium text-fg hover:text-accent"
                  >
                    <Badge variant="secondary" className="mr-2">
                      {eventLabel(run.event)}
                    </Badge>
                    {run.branch}
                  </a>
                  <span className="shrink-0 text-xs text-muted">{formatDate(run.startedAt)}</span>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-1.5">
                  {run.status === 'completed' ? (
                    <>
                      <Badge variant={conclusionVariant(run.conclusion)}>{run.conclusion}</Badge>
                      <span className="text-xs text-muted">{formatDuration(run.durationMs)}</span>
                    </>
                  ) : (
                    <Badge variant="secondary">{run.status}</Badge>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
