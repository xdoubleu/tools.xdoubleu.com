'use client'

import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { MenuItem } from '@/components/ui/menu-item'
import type { FoliateTocItem } from '@/lib/books/foliate'
import { cn } from '@/lib/cn'

interface ReaderTocDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  toc: FoliateTocItem[]
  currentHref?: string
  onSelect: (href: string) => void
}

function TocList({
  items,
  depth,
  currentHref,
  onSelect
}: {
  items: FoliateTocItem[]
  depth: number
  currentHref?: string
  onSelect: (href: string) => void
}) {
  return (
    <ul className={cn(depth > 0 && 'ml-4 border-l border-border')}>
      {items.map(({ label, href, subitems }, i) => (
        <li key={i}>
          {href === undefined ? (
            <p className="px-4 py-2 text-sm text-muted">{label}</p>
          ) : (
            <MenuItem
              aria-current={href === currentHref ? 'location' : undefined}
              className={cn(href === currentHref && 'font-semibold text-accent')}
              onClick={() => onSelect(href)}
            >
              {label}
            </MenuItem>
          )}
          {subitems && subitems.length > 0 && (
            <TocList
              items={subitems}
              depth={depth + 1}
              currentHref={currentHref}
              onSelect={onSelect}
            />
          )}
        </li>
      ))}
    </ul>
  )
}

export default function ReaderTocDrawer({
  open,
  onOpenChange,
  toc,
  currentHref,
  onSelect
}: ReaderTocDrawerProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="right" className="flex flex-col">
        <DialogHeader>
          <DialogTitle>Contents</DialogTitle>
          <DialogClose aria-label="Close contents" />
        </DialogHeader>
        <nav aria-label="Table of contents" className="-mx-2 mt-2">
          <TocList items={toc} depth={0} currentHref={currentHref} onSelect={onSelect} />
        </nav>
      </DialogContent>
    </Dialog>
  )
}
