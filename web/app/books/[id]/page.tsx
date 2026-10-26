import { useRouteParams } from '@/hooks/navigation'
import BookDetailClient from './BookDetailClient'

export default function BookDetailPage() {
  const { id } = useRouteParams<{ id: string }>()
  return <BookDetailClient id={id} />
}
