'use client'

import { useState, useCallback, useRef } from 'react'
import { useSearchExternal, useImportBooks } from '@/hooks/useBacklog'
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
  const [importStatus, setImportStatus] = useState<string | null>(null)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const searchExternal = useSearchExternal()
  const importBooks = useImportBooks()

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

  const handleImport = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0]
      if (!file) return
      setImportStatus('Importing...')
      const reader = new FileReader()
      reader.onload = async (ev) => {
        const csvData = ev.target?.result
        if (typeof csvData !== 'string') return
        try {
          const res = await importBooks(csvData)
          setImportStatus(`Imported ${res.importedCount} book(s).`)
          onAdded()
        } catch {
          setImportStatus('Import failed.')
        }
      }
      reader.readAsText(file)
      e.target.value = ''
    },
    [importBooks, onAdded]
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

      <div className="flex items-center gap-2">
        <label className="inline-flex h-8 cursor-pointer items-center rounded-xl border border-border bg-surface px-3 text-xs text-fg transition-colors hover:bg-hover">
          Import CSV
          <input type="file" accept=".csv" onChange={handleImport} className="hidden" />
        </label>
        {importStatus && <span className="text-sm text-muted">{importStatus}</span>}
      </div>

      {selectedBook && (
        <BookModal book={selectedBook} onClose={() => setSelectedBook(null)} onAdded={onAdded} />
      )}
    </div>
  )
}
