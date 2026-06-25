import AuthorBooksClient from '@/components/backlog/AuthorBooksClient'

export default async function AuthorBooksPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params
  const decoded = decodeURIComponent(name)
  return <AuthorBooksClient name={decoded} />
}
