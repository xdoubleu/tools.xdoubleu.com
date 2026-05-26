import EditFeedClient from './EditFeedClient'

interface EditFeedPageProps {
  params: Promise<{ token: string }>
}

export default async function EditFeedPage({ params }: EditFeedPageProps) {
  const { token } = await params
  return <EditFeedClient token={token} />
}
