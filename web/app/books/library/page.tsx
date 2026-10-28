import BooksSection from '@/components/books/BooksSection'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { PageContainer } from '@/components/ui/page-container'
import LibraryAdminButton from '@/components/books/LibraryAdminButton'

export default function BacklogBooksLibraryPage() {
  return (
    <PageContainer>
      <PageHeader
        breadcrumb={[{ label: 'Books', href: '/dashboard/reading' }, { label: 'Library' }]}
        title="Library"
        actions={
          <>
            <LibraryAdminButton />
            <PageHeaderSettingsLink href="/books/settings" />
          </>
        }
      />

      <BooksSection />
    </PageContainer>
  )
}
