import BooksSettingsClient from '@/components/books/BooksSettingsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { KoboService } from '@/lib/gen/books/v1/kobo_pb'

export default async function BacklogBooksSettingsPage() {
  const client = await createServerClient(KoboService)
  const devices = await fetchOrNull(() => client.listKoboDevices({}))

  return (
    <SWRFallback fallback={devices ? { [swrKeys.koboDevices]: devices } : {}}>
      <BooksSettingsClient />
    </SWRFallback>
  )
}
