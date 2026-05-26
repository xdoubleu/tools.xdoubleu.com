import MealPlanClient from './MealPlanClient'

export default async function MealPlanPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <MealPlanClient id={id} />
}
