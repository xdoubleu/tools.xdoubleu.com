import FamilyPageClient from '@/components/family/FamilyPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FamilyService } from '@/lib/gen/family/v1/family_pb'

export default async function FamilyPage() {
  const client = await createServerClient(FamilyService)
  const family = await fetchOrNull(() => client.getFamily({}))

  return (
    <SWRFallback fallback={family ? { [swrKeys.family]: family } : {}}>
      <FamilyPageClient />
    </SWRFallback>
  )
}
