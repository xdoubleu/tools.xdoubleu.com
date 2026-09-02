'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressForm from '@/components/books/BookProgressForm'

interface BookProgressEditorProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookProgressEditor({ userBook, onSaved }: BookProgressEditorProps) {
  const [editing, setEditing] = useState(false)

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => setEditing(true)}
        aria-label="Edit reading progress"
        className="block w-full text-left py-2 -my-2 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent rounded-lg"
      >
        <BookProgressBar userBook={userBook} />
      </button>
    )
  }

  return (
    <BookProgressForm userBook={userBook} onSaved={onSaved} onClose={() => setEditing(false)} />
  )
}
