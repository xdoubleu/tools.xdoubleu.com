'use client'

import { SectionCard } from '@/components/ui/section-card'
import { ConnectionRow } from '@/components/ui/connection-row'
import { LoadingState } from '@/components/ui/states'
import { Button } from '@/components/ui/button'
import { useDisconnectOAuthConnection } from '@/hooks/useMonitoring'
import type { ListOAuthConnectionsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatDateTime } from '@/lib/dates'
import { getApiUrl } from '@/lib/env'
import ProviderConfigDialog from '@/components/monitoring/ProviderConfigDialog'

const PROVIDER_LABELS: Record<string, string> = {
  github: 'GitHub',
  sentry: 'Sentry'
}

interface OAuthConnectionsCardProps {
  data?: ListOAuthConnectionsResponse
  configuringProvider: string | null
  onConfiguringProviderChange: (provider: string | null) => void
}

export default function OAuthConnectionsCard({
  data,
  configuringProvider,
  onConfiguringProviderChange
}: OAuthConnectionsCardProps) {
  const disconnect = useDisconnectOAuthConnection()

  return (
    <SectionCard
      title="Integrations"
      description="Connect GitHub and Sentry via OAuth so this dashboard can read their data."
    >
      {!data ? (
        <LoadingState className="py-8 text-center text-sm" />
      ) : (
        <ul className="space-y-2">
          {data.connections.map((c) => (
            <li key={c.provider}>
              <ConnectionRow
                name={PROVIDER_LABELS[c.provider] ?? c.provider}
                connected={c.connected}
                detail={
                  c.connected
                    ? `By ${c.connectedBy} on ${formatDateTime(c.connectedAt)}`
                    : undefined
                }
                actions={
                  c.connected ? (
                    <>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => onConfiguringProviderChange(c.provider)}
                      >
                        Configure
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => disconnect(c.provider)}
                      >
                        Disconnect
                      </Button>
                    </>
                  ) : (
                    <Button asChild variant="secondary" size="sm">
                      <a href={`${getApiUrl()}/admin/oauth/${c.provider}/start`}>Connect</a>
                    </Button>
                  )
                }
              />
            </li>
          ))}
        </ul>
      )}

      {configuringProvider && (
        <ProviderConfigDialog
          provider={configuringProvider}
          open
          onOpenChange={(open) => {
            if (!open) onConfiguringProviderChange(null)
          }}
        />
      )}
    </SectionCard>
  )
}
