import PlansListClient from '@/components/recipes/PlansListClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'

export default async function PlansPage() {
  const client = await createServerClient(MealPlansService)
  const plans = await fetchOrNull(() => client.listPlans({}))

  return (
    <SWRFallback fallback={plans ? { [swrKeys.mealPlans]: plans } : {}}>
      <PlansListClient />
    </SWRFallback>
  )
}
