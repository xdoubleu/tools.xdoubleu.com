import { useState, type ReactNode } from 'react'
import { Ellipsis } from 'lucide-react'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'
import { Popover } from '@/components/ui/popover'
import { cn } from '@/lib/cn'

interface DetailHeaderMenuItem {
  label: string
  onSelect: () => void
  /** Delete and remove entries; open a `ConfirmDialog`, never act directly. */
  destructive?: boolean
}

interface DetailHeaderProps {
  breadcrumb: BreadcrumbItem[]
  title: string
  /** Muted line under the title, e.g. an original title. */
  subtitle?: string
  /** An `Artwork`; beside the title from `sm`, above it on phones. */
  artwork?: ReactNode
  /** One muted line of facts. */
  meta?: ReactNode
  /** The item's one status control (`SegmentedTabs`, or a `Select` past five values). */
  status?: ReactNode
  /** Favourite toggle, next to the title. */
  favourite?: ReactNode
  /** Bordered header actions such as Edit. */
  actions?: ReactNode
  /** Overflow menu for delete and remove. */
  menu?: DetailHeaderMenuItem[]
  className?: string
}

// Stryker disable StringLiteral: class names are presentation.
function OverflowMenu({ items }: { items: DetailHeaderMenuItem[] }) {
  const [open, setOpen] = useState(false)
  return (
    <Popover
      open={open}
      onOpenChange={setOpen}
      aria-label="More actions"
      trigger={({ open: expanded, onClick }) => (
        <Button
          variant="secondary"
          size="icon"
          aria-label="More actions"
          aria-haspopup="menu"
          aria-expanded={expanded}
          onClick={onClick}
        >
          <Ellipsis aria-hidden className="size-4" />
        </Button>
      )}
    >
      <div role="menu" className="flex min-w-40 flex-col">
        {items.map((item) => (
          <MenuItem
            key={item.label}
            role="menuitem"
            className={item.destructive ? 'text-danger' : undefined}
            onClick={() => {
              setOpen(false)
              item.onSelect()
            }}
          >
            {item.label}
          </MenuItem>
        ))}
      </div>
    </Popover>
  )
}

/**
 * The top of an item's page: breadcrumb, artwork, title, facts, one status control and the
 * actions. Every item page (book, game, movie, recipe, learning path) uses it so artwork,
 * status and delete sit in the same place.
 */
function DetailHeader({
  breadcrumb,
  title,
  subtitle,
  artwork,
  meta,
  status,
  favourite,
  actions,
  menu,
  className
}: DetailHeaderProps) {
  return (
    <header className={cn('mb-6 space-y-3', className)}>
      <Breadcrumb items={breadcrumb} />
      <div className="flex flex-col gap-4 sm:flex-row">
        {artwork}
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0 flex-1 basis-60">
              <h1 className="break-words font-display text-3xl font-medium leading-tight tracking-tight sm:text-4xl">
                {title}
              </h1>
              {subtitle && <p className="mt-1 text-sm text-muted">{subtitle}</p>}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {favourite}
              {actions}
              {menu && menu.length > 0 && <OverflowMenu items={menu} />}
            </div>
          </div>
          {meta && <p className="text-sm text-muted">{meta}</p>}
          {status}
        </div>
      </div>
    </header>
  )
}

// Stryker restore StringLiteral

export { DetailHeader }
export type { DetailHeaderProps, DetailHeaderMenuItem }
