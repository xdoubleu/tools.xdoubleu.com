import { act, renderHook } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema } from '@/lib/gen/books/v1/library_pb'
import { applyFilters, useLibraryTableState } from '@/components/books/useLibraryTableState'

const none = { ownership: new Set<string>(), format: new Set<string>(), kobo: new Set<string>() }

describe('applyFilters', () => {
  it('matches a book with any selected format', () => {
    const epub = create(UserBookSchema, { tags: [], formats: ['epub'] })
    const pdf = create(UserBookSchema, { tags: [], formats: ['pdf'] })
    const filters = { ...none, format: new Set(['epub', 'pdf']) }
    expect(applyFilters([epub, pdf], filters)).toEqual([epub, pdf])
  })
})

describe('useLibraryTableState', () => {
  beforeEach(() => localStorage.clear())

  it.each([
    ['ownership', 'toggleOwnership', 'own-physical'],
    ['format', 'toggleFormat', 'epub'],
    ['kobo', 'toggleKobo', 'kobo-sync']
  ] as const)('persists the %s filter and goes back to page 1', (key, toggle, value) => {
    const { result } = renderHook(() => useLibraryTableState())
    act(() => result.current.setPage(3))
    act(() => result.current[toggle](value))
    expect(result.current.page).toBe(1)
    expect(result.current.filters[key].has(value)).toBe(true)
    expect(JSON.parse(localStorage.getItem(`backlog:library:filter:${key}`) ?? '[]')).toEqual([
      value
    ])
  })

  it('clears every filter and goes back to page 1', () => {
    const { result } = renderHook(() => useLibraryTableState())
    act(() => result.current.toggleFormat('epub'))
    act(() => result.current.setPage(2))
    act(() => result.current.clearFilters())
    expect(result.current.page).toBe(1)
    expect(result.current.filters.format.size).toBe(0)
  })

  it('restores stored filters', () => {
    localStorage.setItem('backlog:library:filter:ownership', JSON.stringify(['own-digital']))
    const { result } = renderHook(() => useLibraryTableState())
    expect(result.current.filters.ownership.has('own-digital')).toBe(true)
  })
})
