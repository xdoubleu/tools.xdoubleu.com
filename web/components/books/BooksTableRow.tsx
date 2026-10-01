'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { TableRow, TableCell } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import BookDescription from '@/components/books/BookDescription'
import type { BookColumn, CellContext } from '@/components/books/booksTableColumns'
import { cn } from '@/lib/cn'

interface BooksTableRowProps {
  userBook: UserBook
  columns: BookColumn[]
  ctx: CellContext
}

/** A book's table row, led by a toggle that expands its description in a row below. */
export default function BooksTableRow({ userBook, columns, ctx }: BooksTableRowProps) {
  const [expanded, setExpanded] = useState(false)
  const title = userBook.book?.title
  const description = userBook.book?.description ?? ''
  const descriptionId = `book-description-${userBook.id}`
  const open = expanded && description !== ''

  return (
    <>
      <TableRow className={cn(open && 'border-b-0')}>
        <TableCell className="w-8 pr-0">
          {description !== '' && (
            <Button
              type="button"
              variant="ghost"
              size="iconSm"
              aria-expanded={open}
              aria-controls={descriptionId}
              aria-label={`${open ? 'Hide' : 'Show'} description of ${title}`}
              onClick={() => setExpanded((prev) => !prev)}
            >
              <span aria-hidden className="text-muted">
                {open ? '▾' : '▸'}
              </span>
            </Button>
          )}
        </TableCell>
        {columns.map((col) => (
          <TableCell key={col.key} className={col.cellClassName}>
            {col.renderCell(userBook, ctx)}
          </TableCell>
        ))}
      </TableRow>
      {open && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={columns.length + 1} className="pb-4 pl-11">
            <BookDescription
              id={descriptionId}
              description={description}
              className="sticky left-11 max-w-[calc(100cqw-3.75rem)]"
            />
          </TableCell>
        </TableRow>
      )}
    </>
  )
}
