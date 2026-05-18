import ViewerClient from './ViewerClient'

export default async function ViewerPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  return <ViewerClient id={id} />
}
