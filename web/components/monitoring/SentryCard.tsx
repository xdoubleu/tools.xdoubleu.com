'use client'

import { useState } from 'react'
import { ConnectError, Code } from '@connectrpc/connect'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose
} from '@/components/ui/dialog'
import type {
  SentryIssue,
  GetSentryIssuesResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'
import { formatDateTime } from '@/lib/dates'
import { useResolveSentryIssue } from '@/hooks/useMonitoring'
import { getApiUrl } from '@/lib/env'

function levelVariant(level: string): 'danger' | 'warn' | 'secondary' {
  switch (level.toLowerCase()) {
    case 'fatal':
    case 'error':
      return 'danger'
    case 'warning':
    case 'warn':
      return 'warn'
    default:
      return 'secondary'
  }
}

function IssueRow({
  issue,
  onReauthRequired
}: {
  issue: SentryIssue
  onReauthRequired: () => void
}) {
  const resolveSentryIssue = useResolveSentryIssue()
  const [isResolving, setIsResolving] = useState(false)

  async function handleResolve() {
    setIsResolving(true)
    try {
      await resolveSentryIssue(issue.id)
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
        onReauthRequired()
      }
    } finally {
      setIsResolving(false)
    }
  }

  return (
    <li className="rounded-lg border border-border bg-surface p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <a
          href={issue.permalink}
          target="_blank"
          rel="noopener noreferrer"
          className="break-words font-medium text-fg hover:text-accent"
        >
          {issue.title}
        </a>
        <div className="flex shrink-0 items-center gap-1">
          {issue.project && <Badge variant="secondary">{issue.project}</Badge>}
          {issue.level && <Badge variant={levelVariant(issue.level)}>{issue.level}</Badge>}
        </div>
      </div>
      {issue.culprit && (
        <p className="mt-1 break-words font-mono text-xs text-muted">{issue.culprit}</p>
      )}
      <div className="mt-2 flex items-center justify-between gap-2 text-xs text-muted">
        <span>{formatCount(issue.count)} events</span>
        <span>{formatDateTime(issue.lastSeen)}</span>
        <Button
          variant="secondary"
          size="sm"
          className="ml-auto"
          onClick={handleResolve}
          disabled={isResolving}
        >
          {isResolving ? 'Resolving…' : 'Resolve'}
        </Button>
      </div>
    </li>
  )
}

export default function SentryCard({ data }: { data?: GetSentryIssuesResponse }) {
  const issues = data?.issues ?? []
  const [reauthRequired, setReauthRequired] = useState(false)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sentry errors</CardTitle>
        <CardDescription>
          {data ? `${formatCount(data.unresolvedCount)} unresolved.` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">Sentry is not configured.</p>
        ) : issues.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No unresolved issues.</p>
        ) : (
          <ul className="space-y-2">
            {issues.map((issue) => (
              <IssueRow
                key={issue.id}
                issue={issue}
                onReauthRequired={() => setReauthRequired(true)}
              />
            ))}
          </ul>
        )}
      </CardContent>

      <Dialog open={reauthRequired} onOpenChange={setReauthRequired}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Sentry needs to be reconnected</DialogTitle>
            <DialogClose aria-label="Close">x</DialogClose>
          </DialogHeader>
          <DialogDescription>
            This action failed because the Sentry connection is missing a permission it now
            requires. Reconnect to grant it — your org/project selection is kept.
          </DialogDescription>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setReauthRequired(false)}>
              Close
            </Button>
            <Button asChild>
              <a href={`${getApiUrl()}/admin/oauth/sentry/start`}>Reconnect Sentry</a>
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
