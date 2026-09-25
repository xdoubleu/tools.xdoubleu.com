'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useOAuthConnections } from '@/hooks/useMonitoring'
import OAuthConnectionsCard from './OAuthConnectionsCard'

export default function MonitoringSettingsClient() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const oauthConnections = useOAuthConnections()
  const [oauthMessage, setOAuthMessage] = useState<{
    tone: 'success' | 'danger'
    text: string
  } | null>(null)
  const [configuringProvider, setConfiguringProvider] = useState<string | null>(null)

  useEffect(() => {
    const connected = searchParams.get('oauth_connected')
    const errored = searchParams.get('oauth_error')
    if (!connected && !errored) return

    if (connected) {
      setOAuthMessage({ tone: 'success', text: `Connected ${connected}.` })
      void oauthConnections.mutate()
      setConfiguringProvider(connected)
    } else if (errored) {
      setOAuthMessage({
        tone: 'danger',
        text: `Failed to connect ${errored}. Check the server logs for details.`
      })
    }

    const params = new URLSearchParams(searchParams)
    params.delete('oauth_connected')
    params.delete('oauth_error')
    router.replace(
      params.size > 0 ? `/monitoring/connections?${params}` : '/monitoring/connections'
    )
    // Once per incoming URL, not on SWR/router identity changes.
  }, [searchParams])

  return (
    <div className="space-y-4">
      {oauthMessage && (
        <p
          className={`text-sm ${oauthMessage.tone === 'success' ? 'text-success' : 'text-danger'}`}
        >
          {oauthMessage.text}
        </p>
      )}
      <OAuthConnectionsCard
        data={oauthConnections.data}
        configuringProvider={configuringProvider}
        onConfiguringProviderChange={setConfiguringProvider}
      />
    </div>
  )
}
