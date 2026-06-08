import Link from 'next/link'
import { cn } from '@/lib/cn'
import { interactiveCardClass } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'

export default function BacklogPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Backlog</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/backlog/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link href="/backlog/books" className={cn(interactiveCardClass, 'block p-6')}>
          <h2 className="text-xl font-semibold">Books</h2>
          <p className="text-sm text-muted mt-1">Search, library and reading progress.</p>
        </Link>
        <Link href="/backlog/steam" className={cn(interactiveCardClass, 'block p-6')}>
          <h2 className="text-xl font-semibold">Games</h2>
          <p className="text-sm text-muted mt-1">Steam backlog, progress and distribution.</p>
        </Link>
      </div>
    </main>
  )
}
