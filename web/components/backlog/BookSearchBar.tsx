'use client'

import { useState, useCallback, useRef } from 'react'
import { useRouter } from 'next/navigation'
import { useSearchLibrary, useSearchExternal } from '@/hooks/useBacklog'
import type { ExternalBookResult } from '@/lib/gen/backlog/v1/books_pb'
import BookModal from '@/components/backlog/BookModal'
import { Input } from '@/components/ui/input'
import { MenuItem } from '@/components/ui/menu-item'

type SearchResults =
  | { kind: 'library'; books: { id: string; book?: { title: string; authors: string[] } | null }[] }
  | { kind: 'external'; results: ExternalBookResult[] }

interface BookSearchBarProps {
  onAdded: () => void
}

export default function BookSearchBar({ onAdded }: BookSearchBarProps) {
  const router = useRouter()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResults | null>(null)
  const [isSearching, setIsSearching] = useState(false)
  const [selectedBook, setSelectedBook] = useState<ExternalBookResult | null>(null)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const searchLibrary = useSearchLibrary()
  const searchExternal = useSearchExternal()

  const handleQueryChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value
      setQuery(value)

      if (debounceTimer.current) clearTimeout(debounceTimer.current)

      if (!value.trim()) {
        setResults(null)
        return
      }

      debounceTimer.current = setTimeout(async () => {
        setIsSearching(true)
        try {
          const libraryResponse = await searchLibrary(value.trim())
          if (libraryResponse.books.length > 0) {
            setResults({ kind: 'library', books: libraryResponse.books })
          } else {
            const externalResponse = await searchExternal(value.trim())
            setResults({ kind: 'external', results: externalResponse.results })
          }
        } catch {
          setResults(null)
        } finally {
          setIsSearching(false)
        }
      }, 300)
    },
    [searchLibrary, searchExternal]
  )

  const hasResults =
    results !== null &&
    (results.kind === 'library' ? results.books.length > 0 : results.results.length > 0)

  return (
    <div className="space-y-3">
      <div className="relative">
        <Input
          type="text"
          value={query}
          onChange={handleQueryChange}
          placeholder="Search books..."
        />
        {isSearching && (
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted">
            Searching...
          </span>
        )}
        {hasResults && (
          <ul className="absolute z-10 w-full mt-1 bg-card border border-border rounded-2xl shadow-elevated max-h-64 overflow-y-auto">
            {results!.kind === 'library'
              ? results!.books.map((ub) => (
                  <li key={ub.id}>
                    <MenuItem
                      onClick={() => {
                        router.push(`/backlog/books/${ub.id}`)
                        setResults(null)
                        setQuery('')
                      }}
                    >
                      <span className="font-medium">{ub.book?.title}</span>
                      {ub.book && ub.book.authors.length > 0 && (
                        <span className="text-muted ml-2">— {ub.book.authors.join(', ')}</span>
                      )}
                    </MenuItem>
                  </li>
                ))
              : results!.results.map((book) => (
                  <li key={`${book.provider}-${book.providerId}`}>
                    <MenuItem
                      onClick={() => {
                        setSelectedBook(book)
                        setResults(null)
                        setQuery('')
                      }}
                    >
                      <span className="font-medium">{book.title}</span>
                      {book.authors.length > 0 && (
                        <span className="text-muted ml-2">— {book.authors.join(', ')}</span>
                      )}
                    </MenuItem>
                  </li>
                ))}
          </ul>
        )}
      </div>

      {selectedBook && (
        <BookModal book={selectedBook} onClose={() => setSelectedBook(null)} onAdded={onAdded} />
      )}
    </div>
  )
}
