import { Link } from '@/components/ui/link'
import { Button } from '@/components/ui/button'
import { getRelease } from '@/lib/env'

export default function Footer() {
  const release = getRelease()
  const year = new Date().getFullYear()

  return (
    <footer className="border-t border-border/60 bg-glass backdrop-blur-xl backdrop-saturate-150 pl-[var(--inset-left)] pr-[var(--inset-right)] pt-1 pb-[calc(0.25rem+env(safe-area-inset-bottom))] text-xs sm:pt-3 sm:pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
      <div className="mx-auto flex flex-wrap items-center justify-center gap-3 sm:gap-4">
        <div className="text-muted">
          © {year}{' '}
          <Button asChild variant="link" className="text-xs font-normal text-muted underline">
            <Link href="https://xdoubleu.com">xdoubleu</Link>
          </Button>
        </div>

        {release && <span className="font-mono text-xs text-muted">{release.substring(0, 7)}</span>}
      </div>
    </footer>
  )
}
