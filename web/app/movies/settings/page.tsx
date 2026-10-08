import MoviesSettingsClient from '@/components/movies/MoviesSettingsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'

export default async function MoviesSettingsPage() {
  const client = await createServerClient(MoviesService)
  const [settings, providers] = await Promise.all([
    fetchOrNull(() => client.getSettings({})),
    fetchOrNull(() => client.listAvailableProviders({}))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(settings ? { [swrKeys.moviesSettings]: settings } : {}),
        ...(providers ? { [swrKeys.moviesProviders]: providers } : {})
      }}
    >
      <MoviesSettingsClient />
    </SWRFallback>
  )
}
