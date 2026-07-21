'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useDisconnectOAuthConnection } from '@/hooks/useMonitoring'
import type { ListOAuthConnectionsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDateTime } from '@/lib/dates'
import { getApiUrl } from '@/lib/env'

const PROVIDER_LABELS: Record<string, string> = {
  github: 'GitHub',
  sentry: 'Sentry',
  digitalocean: 'DigitalOcean'
}

export default function OAuthConnectionsCard({ data }: { data?: ListOAuthConnectionsResponse }) {
  const disconnect = useDisconnectOAuthConnection()

  return (
    <Card>
      <CardHeader>
        <CardTitle>Integrations</CardTitle>
        <CardDescription>
          Connect GitHub, Sentry, and DigitalOcean via OAuth so this dashboard can read their data.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!data ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : (
          <ul className="space-y-2">
            {data.connections.map((c) => (
              <li
                key={c.provider}
                className="flex items-center justify-between gap-3 rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-fg">
                      {PROVIDER_LABELS[c.provider] ?? c.provider}
                    </span>
                    <Badge variant={c.connected ? 'success' : 'secondary'}>
                      {c.connected ? 'Connected' : 'Not connected'}
                    </Badge>
                  </div>
                  {c.connected && (
                    <p className="mt-1 text-xs text-muted">
                      By {c.connectedBy} on {formatDateTime(c.connectedAt)}
                    </p>
                  )}
                </div>
                {c.connected ? (
                  <Button variant="destructive" size="sm" onClick={() => disconnect(c.provider)}>
                    Disconnect
                  </Button>
                ) : (
                  <Button asChild variant="secondary" size="sm">
                    <a href={`${getApiUrl()}/admin/oauth/${c.provider}/start`}>Connect</a>
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
