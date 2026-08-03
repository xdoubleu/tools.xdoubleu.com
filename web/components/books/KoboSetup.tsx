'use client'

import KoboGatewaySetup from '@/components/books/KoboGatewaySetup'
import KoboGatewayDownload from '@/components/books/KoboGatewayDownload'
import { useGatewayStatus } from '@/hooks/useKoboGateway'

/**
 * Kobo setup is entirely gateway-driven: the local kobo-gateway macOS app
 * does the file work over USB. Background-polls for the gateway (see
 * useGatewayStatus) and shows the download card until it's found.
 */
export default function KoboSetup() {
  const { data: status } = useGatewayStatus()

  if (status) {
    return <KoboGatewaySetup status={status} />
  }

  return <KoboGatewayDownload />
}
