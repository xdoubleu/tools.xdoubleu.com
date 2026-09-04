'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { ToggleIconButton } from '@/components/ui/toggle-icon-button'
import { swrKeys } from '@/lib/swrKeys'

interface BookFavouriteButtonProps {
  userBook: UserBook
  onSaved?: () => void
}

export default function BookFavouriteButton({ userBook, onSaved }: BookFavouriteButtonProps) {
  const [favourite, setFavourite] = useState(userBook.tags.includes('favourite'))
  const updateBookStatus = useUpdateBookStatus()

  const handleClick = async () => {
    const newFavourite = !favourite
    const prev = favourite
    setFavourite(newFavourite)
    try {
      await updateBookStatus({
        bookId: userBook.bookId,
        status: userBook.status,
        favourite: newFavourite,
        rating: String(userBook.rating)
      })
      mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setFavourite(prev)
    }
  }

  return (
    <ToggleIconButton
      active={favourite}
      onToggle={handleClick}
      label="Add to favourites"
      activeLabel="Remove from favourites"
    >
      ♥
    </ToggleIconButton>
  )
}
