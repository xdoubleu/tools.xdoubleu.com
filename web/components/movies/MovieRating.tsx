'use client'

import { useState } from 'react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import { RatingStars } from '@/components/ui/rating-stars'

/**
 * The title's star rating, shown once watched. A rating outlives a status
 * change, so a rated title keeps its stars, still clearable.
 */
export default function MovieRating({ entry }: { entry: BacklogEntry }) {
  const { setRating } = useMoviesActions()
  // The clicked rating, shown until the saved entry catches up.
  const [saving, setSaving] = useState<number | null>(null)
  const [failed, setFailed] = useState(false)

  if (entry.status !== 'watched' && entry.rating === undefined) return null

  const change = async (rating: number) => {
    setSaving(rating)
    setFailed(false)
    try {
      await setRating(entry.id, rating)
    } catch {
      setFailed(true)
    } finally {
      setSaving(null)
    }
  }

  return (
    <section className="space-y-1" aria-label="Rating">
      <h2 className="text-sm text-subtle">Rating</h2>
      <RatingStars
        value={saving ?? entry.rating ?? 0}
        size="md"
        disabled={saving !== null}
        onChange={(r) => void change(r)}
      />
      {failed && <p className="text-sm text-danger">Couldn&apos;t save. Try again.</p>}
    </section>
  )
}
