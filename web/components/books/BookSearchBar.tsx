'use client'

import { useEffect, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useSearchLibrary, useSearchExternal } from '@/hooks/useBooks'
import type { ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import { Input } from '@/components/ui/input'
import { MenuItem } from '@/components/ui/menu-item'

// Standalone mode (omit query/onChange): owns its query, searches the
// library, navigates on a hit, else falls back to external providers.
// Controlled mode: a plain input; BooksLibrary renders the results.
interface BookSearchBarProps {
  // Controlled mode: both set together, or both omitted.
  query?: string
  onChange?: (value: string) => void
}

export default function BookSearchBar({ query: controlledQuery, onChange }: BookSearchBarProps) {
  const isControlled = controlledQuery !== undefined

  const router = useRouter()
  const searchLibrary = useSearchLibrary()
  const searchExternal = useSearchExternal()

  const [standaloneQuery, setStandaloneQuery] = useState('')
  // Local input state so typing is instant; the URL commit is debounced.
  const [controlledInput, setControlledInput] = useState(controlledQuery ?? '')
  const query = isControlled ? controlledQuery : standaloneQuery
  const controlledDebounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Sync from URL changes elsewhere (Back/Forward).
  useEffect(() => {
    if (isControlled) setControlledInput(controlledQuery ?? '')
  }, [isControlled, controlledQuery])

  type LibraryHit = { id: string; book?: { title: string; authors: string[] } | null }
  const [libraryHits, setLibraryHits] = useState<LibraryHit[]>([])
  const [externalResults, setExternalResults] = useState<ExternalBookResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (isControlled) return

    if (debounceTimer.current) clearTimeout(debounceTimer.current)
    if (!standaloneQuery.trim()) {
      if (libraryHits.length) setLibraryHits([])
      if (externalResults.length) setExternalResults([])
      if (isSearching) setIsSearching(false)
      return
    }

    debounceTimer.current = setTimeout(async () => {
      setIsSearching(true)
      try {
        const libResp = await searchLibrary(standaloneQuery.trim())
        if (libResp.books.length > 0) {
          setLibraryHits(libResp.books)
          setExternalResults([])
        } else {
          setLibraryHits([])
          // Results without providerId have no detail page, so drop them.
          const extResp = await searchExternal(standaloneQuery.trim())
          setExternalResults(extResp.results.filter((b) => b.providerId))
        }
      } catch {
        setLibraryHits([])
        setExternalResults([])
      } finally {
        setIsSearching(false)
      }
    }, 300)

    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current)
    }
  }, [standaloneQuery, isControlled, searchLibrary, searchExternal])

  function handleInputChange(value: string) {
    if (isControlled) {
      // Each commit is a router.replace() that re-runs the RSC, so debounce it.
      setControlledInput(value)
      if (controlledDebounceTimer.current) clearTimeout(controlledDebounceTimer.current)
      controlledDebounceTimer.current = setTimeout(() => onChange?.(value), 300)
    } else {
      setStandaloneQuery(value)
      setLibraryHits([])
      setExternalResults([])
    }
  }

  if (isControlled) {
    return (
      <Input
        type="text"
        value={controlledInput}
        onChange={(e) => handleInputChange(e.target.value)}
        placeholder="Search books…"
      />
    )
  }

  const showLibraryDropdown = libraryHits.length > 0
  const showExternalDropdown = externalResults.length > 0

  return (
    <div className="space-y-3">
      <div className="relative">
        <Input
          type="text"
          value={query}
          onChange={(e) => handleInputChange(e.target.value)}
          placeholder="Search books…"
        />
        {isSearching && (
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted">
            Searching…
          </span>
        )}

        {(showLibraryDropdown || showExternalDropdown) && (
          <ul className="absolute z-10 mt-1 max-h-64 w-full overflow-y-auto rounded-2xl border border-border bg-card shadow-elevated">
            {showLibraryDropdown
              ? libraryHits.map((ub) => (
                  <li key={ub.id}>
                    <MenuItem
                      onClick={() => {
                        router.push(`/books/${ub.id}`)
                        setLibraryHits([])
                        setStandaloneQuery('')
                      }}
                    >
                      <span className="font-medium">{ub.book?.title}</span>
                      {ub.book && ub.book.authors.length > 0 && (
                        <span className="ml-2 text-muted">— {ub.book.authors.join(', ')}</span>
                      )}
                    </MenuItem>
                  </li>
                ))
              : externalResults.map((book) => (
                  <li key={`${book.provider}-${book.providerId}`}>
                    <MenuItem
                      onClick={() => {
                        router.push(`/books/external/${book.provider}/${book.providerId}`)
                        setExternalResults([])
                        setStandaloneQuery('')
                      }}
                    >
                      <span className="font-medium">{book.title}</span>
                      {book.authors.length > 0 && (
                        <span className="ml-2 text-muted">— {book.authors.join(', ')}</span>
                      )}
                    </MenuItem>
                  </li>
                ))}
          </ul>
        )}
      </div>
    </div>
  )
}
