import PresenterClient from './PresenterClient'

export default async function PresenterPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <PresenterClient id={id} />
}
