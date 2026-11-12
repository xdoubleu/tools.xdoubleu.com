import { useEffect, useState } from 'react'
import Favourites from '@/components/podcasts/Favourites'
import ShowSearchResults from '@/components/podcasts/ShowSearchResults'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

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
          id="podcasts-search"
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

/** The favourites collection; adding a show is a header action. */
export default function PodcastsClient() {
  const [adding, setAdding] = useState(false)
  const addButton = <Button onClick={() => setAdding(true)}>Add a show</Button>

  return (
    <PageContainer>
      <PageHeader title="Podcasts" actions={addButton} />
      <Favourites addAction={addButton} />
      <AddShowDialog open={adding} onOpenChange={setAdding} />
    </PageContainer>
  )
}
