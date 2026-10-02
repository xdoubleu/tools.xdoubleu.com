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
      {items.map((item, i) => {
        const current = currentHref !== undefined && item.href === currentHref
        return (
          <li key={`${item.href}-${i}`}>
            <MenuItem
              aria-current={current ? 'location' : undefined}
              className={cn(current && 'font-semibold text-accent')}
              onClick={() => onSelect(item.href)}
            >
              {item.label}
            </MenuItem>
            {item.subitems && item.subitems.length > 0 && (
              <TocList
                items={item.subitems}
                depth={depth + 1}
                currentHref={currentHref}
                onSelect={onSelect}
              />
            )}
          </li>
        )
      })}
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
