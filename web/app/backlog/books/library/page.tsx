import BooksSection from '@/components/backlog/BooksSection'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function BacklogBooksLibraryPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Books', href: '/backlog/books' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <BooksSection />
    </main>
  )
}
