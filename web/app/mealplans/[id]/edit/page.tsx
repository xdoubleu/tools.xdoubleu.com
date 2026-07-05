import EditPlanClient from './EditPlanClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'

export default async function EditPlanPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(MealPlansService)
  const plan = await fetchOrNull(() => client.getPlan({ id, offset: 0 }))

  return (
    <SWRFallback fallback={plan ? { [swrKeys.mealPlan(id, 0)]: plan } : {}}>
      <EditPlanClient id={id} />
    </SWRFallback>
  )
}
