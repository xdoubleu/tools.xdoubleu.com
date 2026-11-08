import { useState } from 'react'
import { SectionCard } from '@/components/ui/section-card'
import { ConnectionRow } from '@/components/ui/connection-row'
import { LoadingState } from '@/components/ui/states'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import { useAsyncAction } from '@/hooks/useAsyncAction'
import { Alert } from '@/components/ui/alert'
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
  const [disconnecting, setDisconnecting] = useState<string | null>(null)
  const { run, pending, error } = useAsyncAction()

  const confirmDisconnect = (provider: string) =>
    run(
      () => disconnect(provider),
      `Couldn't disconnect ${PROVIDER_LABELS[provider] ?? provider}. Try again.`
    ).finally(() => setDisconnecting(null))

  return (
    <SectionCard
      title="Integrations"
      description="Connect GitHub and Sentry via OAuth so this dashboard can read their data."
    >
      {error && (
        <Alert tone="danger" className="mb-3">
          {error}
        </Alert>
      )}
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
                        onClick={() => setDisconnecting(c.provider)}
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

      <ConfirmDialog
        open={disconnecting !== null}
        onOpenChange={(open) => {
          if (!open) setDisconnecting(null)
        }}
        title={`Disconnect ${disconnecting ? (PROVIDER_LABELS[disconnecting] ?? disconnecting) : ''}?`}
        description="This dashboard stops reading its data until you connect it again."
        confirmLabel="Disconnect"
        pendingLabel="Disconnecting…"
        destructive
        pending={pending}
        onConfirm={() => {
          if (disconnecting) void confirmDisconnect(disconnecting)
        }}
      />

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
