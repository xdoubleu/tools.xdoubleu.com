import BooksSection from '@/components/books/BooksSection'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { PageContainer } from '@/components/ui/page-container'
import LibraryAdminButton from '@/components/books/LibraryAdminButton'
import AddBookButton from '@/components/books/AddBookButton'

export default function BacklogBooksLibraryPage() {
  return (
    <PageContainer size="wide">
      <PageHeader
        breadcrumb={[{ label: 'Books', href: '/dashboard/reading' }, { label: 'Library' }]}
        title="Library"
        actions={
          <>
            <LibraryAdminButton />
            <PageHeaderSettingsLink href="/books/settings" />
            <AddBookButton />
          </>
        }
      />

      <BooksSection />
    </PageContainer>
  )
}
