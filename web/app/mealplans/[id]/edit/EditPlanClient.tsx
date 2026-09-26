'use client'

import { useRouter } from 'next/navigation'
import { useMealPlan } from '@/hooks/useMealPlans'
import PlanForm from '@/components/mealplans/PlanForm'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function EditPlanClient({ id }: { id: string }) {
  const { data, isLoading, error } = useMealPlan(id)
  const router = useRouter()
  const plan = data?.plan

  return (
    <PageContainer size="form">
      <PageHeader
        title="Settings"
        breadcrumb={[
          { label: 'Meal Plans', href: '/mealplans' },
          { label: plan?.name ?? 'Plan', href: `/mealplans/${id}` },
          { label: 'Settings' }
        ]}
      />

      {isLoading && <LoadingState label="plan" />}
      {error && <ErrorState what="plan" />}
      {plan && (
        <PlanForm
          plan={plan}
          onSave={(planId) => router.push(`/mealplans/${planId}`)}
          onCancel={() => router.push(`/mealplans/${id}`)}
        />
      )}
    </PageContainer>
  )
}
