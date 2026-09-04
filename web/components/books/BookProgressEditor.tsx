'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressForm from '@/components/books/BookProgressForm'
import { Button } from '@/components/ui/button'

interface BookProgressEditorProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookProgressEditor({ userBook, onSaved }: BookProgressEditorProps) {
  const [editing, setEditing] = useState(false)

  if (!editing) {
    return (
      <Button
        variant="ghost"
        onClick={() => setEditing(true)}
        aria-label="Edit reading progress"
        className="-my-2 block h-auto w-full rounded-lg px-0 py-2 text-left hover:bg-transparent"
      >
        <BookProgressBar userBook={userBook} />
      </Button>
    )
  }

  return (
    <BookProgressForm userBook={userBook} onSaved={onSaved} onClose={() => setEditing(false)} />
  )
}
