'use client'

import KoboGatewaySetup from '@/components/books/KoboGatewaySetup'
import KoboGatewayDownload from '@/components/books/KoboGatewayDownload'
import { useGatewayStatus } from '@/hooks/useKoboGateway'

/** Gateway-driven Kobo setup; shows the download card until the gateway is found. */
export default function KoboSetup() {
  const { data: status } = useGatewayStatus()

  if (status) {
    return <KoboGatewaySetup status={status} />
  }

  return <KoboGatewayDownload />
}
