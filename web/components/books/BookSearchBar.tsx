'use client'

import { useEffect, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useSearchLibrary, useSearchExternal } from '@/hooks/useBooks'
import type { ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import BookModal from '@/components/books/BookModal'
import { Input } from '@/components/ui/input'
import { MenuItem } from '@/components/ui/menu-item'

// Two usage modes:
//
//  1. Standalone mode (BooksDashboard): omit query/onChange/hasLibraryResults.
//     The bar manages its own query state, searches the library, navigates on
//     a hit, and falls back to Open Library when the library has no results.
//
//  2. Controlled mode (BooksSection / library page): supply query, onChange,
//     and hasLibraryResults.  The bar is a controlled input; it does NOT search
//     the library itself (BooksLibrary does that via client-side filtering).
//     When hasLibraryResults is false it searches Open Library as a fallback.
interface BookSearchBarProps {
  onAdded: () => void
  // Controlled-mode props (all required together, all omitted for standalone).
  query?: string
  onChange?: (value: string) => void
  hasLibraryResults?: boolean
}

export default function BookSearchBar({
  onAdded,
  query: controlledQuery,
  onChange,
  hasLibraryResults
}: BookSearchBarProps) {
  const isControlled = controlledQuery !== undefined

  const router = useRouter()
  const searchLibrary = useSearchLibrary()
  const searchExternal = useSearchExternal()

  // Standalone mode owns its own query state.
  const [standaloneQuery, setStandaloneQuery] = useState('')
  const query = isControlled ? controlledQuery : standaloneQuery

  type LibraryHit = { id: string; book?: { title: string; authors: string[] } | null }
  const [libraryHits, setLibraryHits] = useState<LibraryHit[]>([])
  const [externalResults, setExternalResults] = useState<ExternalBookResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [selectedBook, setSelectedBook] = useState<ExternalBookResult | null>(null)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ---- Standalone mode: debounce → searchLibrary → navigate or OL fallback ----
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
          const extResp = await searchExternal(standaloneQuery.trim())
          setExternalResults(extResp.results)
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

  // ---- Controlled mode: OL fallback when library filter has no results ----
  useEffect(() => {
    if (!isControlled) return

    if (debounceTimer.current) clearTimeout(debounceTimer.current)

    if (!controlledQuery?.trim() || hasLibraryResults) {
      if (externalResults.length) setExternalResults([])
      if (isSearching) setIsSearching(false)
      return
    }

    debounceTimer.current = setTimeout(async () => {
      setIsSearching(true)
      try {
        const resp = await searchExternal(controlledQuery.trim())
        setExternalResults(resp.results)
      } catch {
        setExternalResults([])
      } finally {
        setIsSearching(false)
      }
    }, 300)

    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current)
    }
  }, [controlledQuery, hasLibraryResults, isControlled, searchExternal])

  const showLibraryDropdown = !isControlled && libraryHits.length > 0
  const showExternalDropdown =
    externalResults.length > 0 &&
    (isControlled ? !hasLibraryResults && (controlledQuery?.trim() ?? '') !== '' : true)

  function handleInputChange(value: string) {
    if (isControlled) {
      onChange?.(value)
    } else {
      setStandaloneQuery(value)
      setLibraryHits([])
      setExternalResults([])
    }
  }

  return (
    <div className="space-y-3">
      <div className="relative">
        <Input
          type="text"
          value={query}
          onChange={(e) => handleInputChange(e.target.value)}
          placeholder="Search books..."
        />
        {isSearching && (
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted">
            Searching...
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
                        setSelectedBook(book)
                        setExternalResults([])
                        if (!isControlled) setStandaloneQuery('')
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

      {selectedBook && (
        <BookModal
          book={selectedBook}
          onClose={() => setSelectedBook(null)}
          onAdded={() => {
            setSelectedBook(null)
            onAdded()
          }}
        />
      )}
    </div>
  )
}
