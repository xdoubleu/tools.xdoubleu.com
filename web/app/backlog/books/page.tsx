import BooksSection from '@/components/backlog/BooksSection'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogBooksPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Backlog', href: '/backlog' }, { label: 'Books' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Books</h1>

      <BooksSection />
    </main>
  )
}
