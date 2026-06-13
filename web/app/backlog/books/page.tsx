import BooksDashboard from '@/components/backlog/BooksDashboard'

export default function BacklogBooksPage() {
  return (
    <main className="mx-auto max-w-6xl p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <h1 className="mb-4 text-3xl font-bold lg:mb-3 lg:text-2xl">Books</h1>

      <BooksDashboard />
    </main>
  )
}
