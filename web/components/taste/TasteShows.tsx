import { useEffect, useState } from 'react'
import ShowFavourites from '@/components/taste/ShowFavourites'
import ShowSearchResults from '@/components/taste/ShowSearchResults'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { SectionCard } from '@/components/ui/section-card'

// Each search is an iTunes request; wait for typing to pause.
const SEARCH_DEBOUNCE_MS = 300

/** iTunes search in a sheet; a two-character query fetches results. */
function AddShowDialog({
  open,
  onOpenChange
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  // Clearing the box clears the results at once; only the fetch waits.
  const searching = debouncedQuery.trim().length >= 2 && query.trim().length >= 2

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a show</DialogTitle>
        </DialogHeader>
        <DialogDescription>Search iTunes and favourite a show.</DialogDescription>
        <Input
          id="shows-search"
          type="search"
          aria-label="Search iTunes"
          placeholder="Show name…"
          autoComplete="off"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {searching && <ShowSearchResults query={debouncedQuery} />}
      </DialogContent>
    </Dialog>
  )
}

/** The favourite shows, which feed the podcasts domain; adding one is a header action. */
export default function TasteShows() {
  const [adding, setAdding] = useState(false)

  return (
    <SectionCard
      title="Shows"
      description="Favourite shows count as podcasts you like."
      action={<Button onClick={() => setAdding(true)}>Add a show</Button>}
    >
      <ShowFavourites />
      <AddShowDialog open={adding} onOpenChange={setAdding} />
    </SectionCard>
  )
}
