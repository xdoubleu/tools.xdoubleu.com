import { Link } from '@/components/ui/link'
import { cn } from '@/lib/cn'
import { SectionHeading } from '@/components/ui/section-heading'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { TogglePill } from '@/components/ui/toggle-pill'

export interface SidebarShelf {
  id: string
  label: string
  count: number
}

export interface SidebarTag {
  name: string
  count: number
}

export interface SidebarSeries {
  name: string
  href: string
  read: number
  total: number
}

interface ShelfSidebarProps {
  shelves: SidebarShelf[]
  allTags?: SidebarTag[]
  selectedShelfId: string | null
  selectedTag: string | null
  onSelectShelf: (id: string) => void
  onSelectTag: (tag: string) => void
  /** Omit on read-only views (public profile) to hide shelf/tag editing. */
  onManage?: () => void
  /** Links to each series page; omit where those pages aren't reachable. */
  series?: SidebarSeries[]
}

function NavItem({
  active,
  onClick,
  label,
  count
}: {
  active: boolean
  onClick: () => void
  label: string
  count: number
}) {
  return (
    <MenuItem
      onClick={onClick}
      aria-current={active ? 'true' : undefined}
      className={cn(
        'justify-between rounded-xl px-3 py-2',
        active ? 'bg-accent/10 text-accent font-medium' : 'text-subtle hover:text-fg'
      )}
    >
      <span className="truncate">{label}</span>
      <span className="ml-2 text-xs text-muted shrink-0">{count}</span>
    </MenuItem>
  )
}

export function ShelfSidebar({
  shelves,
  allTags = [],
  selectedShelfId,
  selectedTag,
  onSelectShelf,
  onSelectTag,
  onManage,
  series = []
}: ShelfSidebarProps) {
  return (
    <>
      <nav
        className="hidden md:flex flex-col gap-1 min-w-44 shrink-0"
        aria-label="Library navigation"
      >
        <SectionHeading level="eyebrow" className="px-3 py-1">
          Shelves
        </SectionHeading>
        {shelves.map((shelf) => (
          <NavItem
            key={shelf.id}
            active={selectedShelfId === shelf.id}
            onClick={() => onSelectShelf(shelf.id)}
            label={shelf.label}
            count={shelf.count}
          />
        ))}

        {allTags.length > 0 && (
          <>
            <div className="my-1 h-px bg-border" />
            <SectionHeading level="eyebrow" className="px-3 py-1">
              Tags
            </SectionHeading>
            {allTags.map((tag) => (
              <NavItem
                key={tag.name}
                active={selectedTag === tag.name}
                onClick={() => onSelectTag(tag.name)}
                label={tag.name}
                count={tag.count}
              />
            ))}
          </>
        )}

        {series.length > 0 && (
          <>
            <div className="my-1 h-px bg-border" />
            <SectionHeading level="eyebrow" className="px-3 py-1">
              Series
            </SectionHeading>
            {series.map((s) => (
              <Button
                key={s.name}
                asChild
                variant="ghost"
                className="justify-between rounded-xl px-3 font-normal text-subtle hover:text-fg"
              >
                <Link href={s.href}>
                  <span className="truncate">{s.name}</span>
                  <span className="ml-2 shrink-0 text-xs text-muted">
                    {s.read}/{s.total}
                  </span>
                </Link>
              </Button>
            ))}
          </>
        )}

        {onManage && (
          <>
            <div className="my-1 h-px bg-border" />
            <MenuItem onClick={onManage} className="rounded-xl px-3 py-2 text-subtle hover:text-fg">
              Edit shelves &amp; tags
            </MenuItem>
          </>
        )}
      </nav>

      <div className="flex md:hidden flex-col gap-2">
        <SegmentedTabs<string>
          aria-label="Shelves"
          value={selectedShelfId}
          onChange={onSelectShelf}
          options={shelves.map((shelf) => ({
            value: shelf.id,
            label: (
              <>
                {shelf.label}
                <span className="ml-1 text-xs opacity-60">{shelf.count}</span>
              </>
            )
          }))}
          className="-mx-1 flex-wrap px-1 pb-1"
        />
        {allTags.length > 0 && (
          <div className="flex flex-wrap gap-2 px-1 pb-1" aria-label="Tags">
            {allTags.map((tag) => (
              <TogglePill
                key={tag.name}
                active={selectedTag === tag.name}
                onClick={() => onSelectTag(tag.name)}
                label={
                  <>
                    {tag.name}
                    <span className="ml-1 opacity-60">{tag.count}</span>
                  </>
                }
                className="shrink-0 px-2 py-1 whitespace-nowrap"
              />
            ))}
          </div>
        )}
        {series.length > 0 && (
          <div className="flex flex-wrap gap-2 px-1 pb-1" aria-label="Series">
            {series.map((s) => (
              <Button key={s.name} asChild variant="secondary" size="sm" className="max-w-full">
                <Link href={s.href}>
                  <span className="truncate">{s.name}</span>
                  <span className="ml-1 shrink-0 opacity-60">
                    {s.read}/{s.total}
                  </span>
                </Link>
              </Button>
            ))}
          </div>
        )}
      </div>
    </>
  )
}
