import EditPlanClient from './EditPlanClient'

export default async function EditPlanPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <EditPlanClient id={id} />
}
