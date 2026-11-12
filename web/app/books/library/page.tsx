import AppSections from '@/components/AppSections'
import BooksSection from '@/components/books/BooksSection'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { PageContainer } from '@/components/ui/page-container'
import LibraryAdminButton from '@/components/books/LibraryAdminButton'
import AddBookButton from '@/components/books/AddBookButton'

export default function BacklogBooksLibraryPage() {
  return (
    <PageContainer size="wide">
      <PageHeader
        title="Books library"
        actions={
          <>
            <LibraryAdminButton />
            <PageHeaderSettingsLink href="/books/settings" />
            <AddBookButton />
          </>
        }
      />

      <AppSections app="books" />

      <BooksSection />
    </PageContainer>
  )
}
