import BooksSection from '@/components/books/BooksSection'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function BacklogBooksLibraryPage() {
  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Books', href: '/books' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <BooksSection />
    </PageContainer>
  )
}
