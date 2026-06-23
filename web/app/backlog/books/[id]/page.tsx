import BookDetailClient from './BookDetailClient'

export default async function BookDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  return <BookDetailClient id={id} />
}
