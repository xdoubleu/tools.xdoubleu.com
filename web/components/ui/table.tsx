import { type HTMLAttributes, type TdHTMLAttributes, type ThHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

function Table({ className, ...props }: HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="w-full overflow-x-auto rounded-2xl border border-border bg-card">
      <table className={cn('w-full caption-bottom text-sm', className)} {...props} />
    </div>
  )
}

function TableHeader({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn('[&_tr]:border-b [&_tr]:border-border', className)} {...props} />
}

function TableBody({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={cn('[&_tr:last-child]:border-0', className)} {...props} />
}

function TableRow({ className, ...props }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr
      className={cn(
        'border-b border-border transition-colors hover:bg-hover data-[selected=true]:bg-accent/5',
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={cn(
        'h-10 px-3 text-left align-middle text-xs font-medium text-muted',
        'has-[[role=checkbox]]:pr-0',
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td className={cn('px-3 py-2 align-middle has-[[role=checkbox]]:pr-0', className)} {...props} />
  )
}

// SortableHeader renders a column header button with an asc/desc/none indicator.
type SortDir = 'asc' | 'desc' | null

interface SortableHeaderProps extends Omit<ThHTMLAttributes<HTMLTableCellElement>, 'dir'> {
  dir: SortDir
  onSort: () => void
}

function SortableHeader({ dir, onSort, children, className, ...props }: SortableHeaderProps) {
  const indicator = dir === 'asc' ? ' ^' : dir === 'desc' ? ' v' : ''
  return (
    <TableHead className={cn('cursor-pointer select-none', className)} {...props}>
      <button
        type="button"
        onClick={onSort}
        className="flex items-center gap-1 text-xs font-medium text-muted hover:text-foreground transition-colors whitespace-nowrap"
      >
        {children}
        {indicator && (
          <span className="text-accent font-bold" aria-hidden="true">
            {indicator}
          </span>
        )}
      </button>
    </TableHead>
  )
}

export { Table, TableHeader, TableBody, TableRow, TableHead, TableCell, SortableHeader }
export type { SortDir }
