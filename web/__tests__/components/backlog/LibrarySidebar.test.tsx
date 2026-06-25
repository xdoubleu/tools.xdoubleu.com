import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type Shelf
} from '@/components/backlog/LibrarySidebar'
import { create } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  BookSchema,
  BookShelfSchema
} from '@/lib/gen/backlog/v1/books_pb'

function makeBook(id: string, tags: string[] = []) {
  return create(UserBookSchema, {
    id,
    status: 'to-read',
    tags,
    book: create(BookSchema, { title: `Book ${id}`, authors: [] })
  })
}

function makeLibrary() {
  return create(LibraryResponseSchema, {
    reading: [makeBook('r1')],
    wishlist: [makeBook('w1', ['fantasy'])],
    finished: [makeBook('f1', ['sci-fi'])],
    shelves: [create(BookShelfSchema, { name: 'Custom', books: [makeBook('c1')] })]
  })
}

describe('buildShelves', () => {
  it('returns All, built-ins, and custom shelves', () => {
    const library = makeLibrary()
    const shelves = buildShelves(library)
    const ids = shelves.map((s) => s.id)
    expect(ids).toContain('all')
    expect(ids).toContain('currently-reading')
    expect(ids).toContain('wishlist')
    expect(ids).toContain('finished')
    expect(ids).toContain('Custom')
  })

  it('counts books correctly', () => {
    const library = makeLibrary()
    const shelves = buildShelves(library)
    const all = shelves.find((s) => s.id === 'all')!
    expect(all.count).toBe(4)
    const reading = shelves.find((s) => s.id === 'currently-reading')!
    expect(reading.count).toBe(1)
  })
})

describe('buildTags', () => {
  it('returns sorted non-special tags from all books', () => {
    const library = makeLibrary()
    const tags = buildTags(library)
    expect(tags).toContain('fantasy')
    expect(tags).toContain('sci-fi')
    expect(tags).not.toContain('favourite')
  })

  it('deduplicates tags', () => {
    const library = create(LibraryResponseSchema, {
      reading: [makeBook('a', ['fantasy']), makeBook('b', ['fantasy'])],
      wishlist: [],
      finished: [],
      shelves: []
    })
    const tags = buildTags(library)
    expect(tags.filter((t) => t === 'fantasy')).toHaveLength(1)
  })
})

describe('LibrarySidebar', () => {
  const shelves: Shelf[] = [
    { id: 'all', label: 'All books', count: 5 },
    { id: 'currently-reading', label: 'Currently reading', count: 2 },
    { id: 'wishlist', label: 'Want to read', count: 3 }
  ]

  it('renders all shelves in the desktop nav', () => {
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={[]}
        selectedShelf="all"
        selectedTags={new Set()}
        onSelectShelf={jest.fn()}
        onToggleTag={jest.fn()}
        onManage={jest.fn()}
      />
    )
    expect(screen.getAllByText('All books').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Currently reading').length).toBeGreaterThan(0)
  })

  it('calls onSelectShelf when a shelf button is clicked', () => {
    const onSelectShelf = jest.fn()
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={[]}
        selectedShelf="all"
        selectedTags={new Set()}
        onSelectShelf={onSelectShelf}
        onToggleTag={jest.fn()}
        onManage={jest.fn()}
      />
    )
    // Desktop nav buttons
    const btns = screen.getAllByText('Want to read')
    fireEvent.click(btns[0])
    expect(onSelectShelf).toHaveBeenCalledWith('wishlist')
  })

  it('renders tags and calls onToggleTag when clicked', () => {
    const onToggleTag = jest.fn()
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={['fantasy', 'sci-fi']}
        selectedShelf="all"
        selectedTags={new Set()}
        onSelectShelf={jest.fn()}
        onToggleTag={onToggleTag}
        onManage={jest.fn()}
      />
    )
    // Click the desktop tag button
    const btns = screen.getAllByText('fantasy')
    fireEvent.click(btns[0])
    expect(onToggleTag).toHaveBeenCalledWith('fantasy')
  })

  it('calls onManage when Edit shelves & tags is clicked', () => {
    const onManage = jest.fn()
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={[]}
        selectedShelf="all"
        selectedTags={new Set()}
        onSelectShelf={jest.fn()}
        onToggleTag={jest.fn()}
        onManage={onManage}
      />
    )
    fireEvent.click(screen.getByText('Edit shelves & tags'))
    expect(onManage).toHaveBeenCalledTimes(1)
  })
})
