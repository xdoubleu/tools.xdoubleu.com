import Link from 'next/link'
import BooksDashboard from '@/components/books/BooksDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'

export default function BacklogBooksPage() {
  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <div className="mb-4 flex items-center justify-between gap-4 lg:mb-3">
        <h1 className="text-3xl font-bold lg:text-2xl">Books</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/books/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <BooksDashboard />
    </PageContainer>
  )
}
