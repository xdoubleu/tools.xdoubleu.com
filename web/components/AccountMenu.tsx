import { useState } from 'react'
import { CircleUser } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Link } from '@/components/ui/link'
import { MenuItem } from '@/components/ui/menu-item'
import { Popover } from '@/components/ui/popover'
import { SectionHeading } from '@/components/ui/section-heading'
import { useSignOut } from '@/hooks/useAuth'
import { cn } from '@/lib/cn'
import { flushOutbox } from '@/lib/offline/outbox'
import { clearOfflineData } from '@/lib/offline/session'
import { getTheme, setTheme, THEMES, type Theme } from '@/lib/theme'

// Stryker disable StringLiteral: class names and menu copy are presentation; tests assert the labels.
const THEME_LABELS: Record<Theme, string> = { auto: 'Auto', light: 'Light', dark: 'Dark' }

const linkItemClass =
  'flex w-full items-center rounded-lg px-4 py-2 text-left text-sm text-fg min-h-11 sm:min-h-9 hover:bg-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent'

/** Settings, family, appearance and sign out, in one menu. */
export default function AccountMenu({
  align = 'right',
  className
}: {
  align?: 'left' | 'right'
  className?: string
}) {
  const signOut = useSignOut()
  const [open, setOpen] = useState(false)
  const [theme, setThemeState] = useState<Theme>(getTheme)

  const handleSignOut = async () => {
    // Send queued writes first; sign-out deletes whatever is left.
    await flushOutbox()
    await signOut()
    await clearOfflineData()
    window.location.href = '/'
  }

  return (
    <Popover
      open={open}
      onOpenChange={setOpen}
      align={align}
      aria-label="Account"
      className="w-56 p-1"
      trigger={({ open: isOpen, onClick }) => (
        <Button
          variant="ghost"
          size="icon"
          aria-label="Account menu"
          aria-expanded={isOpen}
          onClick={onClick}
          className={cn('text-muted hover:text-fg', className)}
        >
          <CircleUser aria-hidden className="size-5 shrink-0" />
        </Button>
      )}
    >
      <div role="menu" aria-label="Account">
        <Link
          href="/settings"
          role="menuitem"
          className={linkItemClass}
          onClick={() => setOpen(false)}
        >
          Account settings
        </Link>
        <Link
          href="/family"
          role="menuitem"
          className={linkItemClass}
          onClick={() => setOpen(false)}
        >
          Family
        </Link>
        <div role="group" aria-label="Appearance" className="my-1 border-t border-border/60 py-1">
          <SectionHeading level="eyebrow" as="h3" className="px-4 pb-1 pt-2">
            Appearance
          </SectionHeading>
          {THEMES.map((value) => (
            <MenuItem
              key={value}
              role="menuitemradio"
              aria-checked={theme === value}
              className={cn(theme === value && 'font-medium text-accent')}
              onClick={() => setThemeState(setTheme(value))}
            >
              {THEME_LABELS[value]}
            </MenuItem>
          ))}
        </div>
        <div className="border-t border-border/60 pt-1">
          <MenuItem role="menuitem" onClick={handleSignOut}>
            Sign out
          </MenuItem>
        </div>
      </div>
    </Popover>
  )
}
// Stryker restore StringLiteral
