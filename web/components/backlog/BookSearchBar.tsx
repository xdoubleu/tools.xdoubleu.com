'use client'

import { useState, useCallback, useRef } from 'react'
import { useSearchExternal } from '@/hooks/useBacklog'
import type { ExternalBookResult } from '@/lib/gen/backlog/v1/books_pb'
import BookModal from '@/components/backlog/BookModal'
import { Input } from '@/components/ui/input'
import { MenuItem } from '@/components/ui/menu-item'

interface BookSearchBarProps {
  onAdded: () => void
}

export default function BookSearchBar({ onAdded }: BookSearchBarProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<ExternalBookResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [selectedBook, setSelectedBook] = useState<ExternalBookResult | null>(null)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const searchExternal = useSearchExternal()

  const handleQueryChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value
      setQuery(value)

      if (debounceTimer.current) clearTimeout(debounceTimer.current)

      if (!value.trim()) {
        setResults([])
        return
      }

      debounceTimer.current = setTimeout(async () => {
        setIsSearching(true)
        try {
          const response = await searchExternal(value.trim())
          setResults(response.results)
        } catch {
          setResults([])
        } finally {
          setIsSearching(false)
        }
      }, 300)
    },
    [searchExternal]
  )

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
        {results.length > 0 && (
          <ul className="absolute z-10 w-full mt-1 bg-card border border-border rounded-2xl shadow-elevated max-h-64 overflow-y-auto">
            {results.map((book) => (
              <li key={`${book.provider}-${book.providerId}`}>
                <MenuItem
                  onClick={() => {
                    setSelectedBook(book)
                    setResults([])
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
