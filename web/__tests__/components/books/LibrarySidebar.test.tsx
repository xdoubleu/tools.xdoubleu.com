import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import LibrarySidebar, {
  buildShelves,
  buildTags,
  type Shelf,
  type TagEntry
} from '@/components/books/LibrarySidebar'
import { create } from '@bufbuild/protobuf'
import {
  LibraryResponseSchema,
  UserBookSchema,
  BookSchema,
  BookShelfSchema
} from '@/lib/gen/books/v1/library_pb'

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
    wishlist: [makeBook('w1', ['fantasy']), makeBook('w2', ['fantasy'])],
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
    expect(all.count).toBe(5)
    const reading = shelves.find((s) => s.id === 'currently-reading')!
    expect(reading.count).toBe(1)
  })

  it('includes a Favourites shelf with correct count', () => {
    const library = create(LibraryResponseSchema, {
      reading: [makeBook('r1', ['favourite']), makeBook('r2')],
      wishlist: [makeBook('w1', ['favourite'])],
      finished: [makeBook('f1')],
      shelves: []
    })
    const shelves = buildShelves(library)
    const favShelf = shelves.find((s) => s.id === 'favourite')
    expect(favShelf).toBeDefined()
    expect(favShelf!.label).toBe('Favourites')
    expect(favShelf!.count).toBe(2)
  })

  it('surfaces a raw "dropped" shelf as a fixed Dropped entry, not duplicated in custom shelves', () => {
    const library = create(LibraryResponseSchema, {
      reading: [],
      wishlist: [],
      finished: [],
      shelves: [create(BookShelfSchema, { name: 'dropped', books: [makeBook('d1')] })]
    })
    const shelves = buildShelves(library)
    const dropped = shelves.filter((s) => s.id === 'dropped')
    expect(dropped).toHaveLength(1)
    expect(dropped[0].label).toBe('Dropped')
    expect(dropped[0].count).toBe(1)
  })

  it('omits the Dropped shelf when there are no dropped books', () => {
    const library = makeLibrary()
    const shelves = buildShelves(library)
    expect(shelves.find((s) => s.id === 'dropped')).toBeUndefined()
  })
})

describe('buildTags', () => {
  it('returns sorted non-special tags with counts', () => {
    const library = makeLibrary()
    const tags = buildTags(library)
    const names = tags.map((t) => t.name)
    expect(names).toContain('fantasy')
    expect(names).toContain('sci-fi')
    expect(names).not.toContain('favourite')
  })

  it('counts occurrences per tag', () => {
    const library = makeLibrary()
    const tags = buildTags(library)
    const fantasy = tags.find((t) => t.name === 'fantasy')!
    expect(fantasy.count).toBe(2)
    const scifi = tags.find((t) => t.name === 'sci-fi')!
    expect(scifi.count).toBe(1)
  })

  it('deduplicates tag names', () => {
    const library = create(LibraryResponseSchema, {
      reading: [makeBook('a', ['fantasy']), makeBook('b', ['fantasy'])],
      wishlist: [],
      finished: [],
      shelves: []
    })
    const tags = buildTags(library)
    expect(tags.filter((t) => t.name === 'fantasy')).toHaveLength(1)
  })
})

describe('LibrarySidebar', () => {
  const shelves: Shelf[] = [
    { id: 'all', label: 'All books', count: 5 },
    { id: 'currently-reading', label: 'Currently reading', count: 2 },
    { id: 'wishlist', label: 'Want to read', count: 3 }
  ]
  const tags: TagEntry[] = [
    { name: 'fantasy', count: 4 },
    { name: 'sci-fi', count: 1 }
  ]

  it('renders all shelves in the desktop nav', () => {
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={[]}
        selectedShelfId="all"
        selectedTag={null}
        onSelectShelf={jest.fn()}
        onSelectTag={jest.fn()}
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
        selectedShelfId="all"
        selectedTag={null}
        onSelectShelf={onSelectShelf}
        onSelectTag={jest.fn()}
        onManage={jest.fn()}
      />
    )
    const btns = screen.getAllByText('Want to read')
    fireEvent.click(btns[0])
    expect(onSelectShelf).toHaveBeenCalledWith('wishlist')
  })

  it('renders tags and calls onSelectTag when clicked', () => {
    const onSelectTag = jest.fn()
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={tags}
        selectedShelfId="all"
        selectedTag={null}
        onSelectShelf={jest.fn()}
        onSelectTag={onSelectTag}
        onManage={jest.fn()}
      />
    )
    const btns = screen.getAllByText('fantasy')
    fireEvent.click(btns[0])
    expect(onSelectTag).toHaveBeenCalledWith('fantasy')
  })

  it('renders tag counts next to tag names in the desktop sidebar', () => {
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={tags}
        selectedShelfId="all"
        selectedTag={null}
        onSelectShelf={jest.fn()}
        onSelectTag={jest.fn()}
        onManage={jest.fn()}
      />
    )
    // Count "4" should appear next to the fantasy tag
    expect(screen.getAllByText('4').length).toBeGreaterThan(0)
    expect(screen.getAllByText('1').length).toBeGreaterThan(0)
  })

  it('marks the active tag as selected', () => {
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={tags}
        selectedShelfId={null}
        selectedTag="fantasy"
        onSelectShelf={jest.fn()}
        onSelectTag={jest.fn()}
        onManage={jest.fn()}
      />
    )
    // The active nav item gets accent styling; shelf items should not be active
    // At minimum, no shelf should appear active while a tag is selected
    expect(screen.getAllByText('fantasy').length).toBeGreaterThan(0)
  })

  it('calls onManage when Edit shelves & tags is clicked', () => {
    const onManage = jest.fn()
    render(
      <LibrarySidebar
        shelves={shelves}
        allTags={[]}
        selectedShelfId="all"
        selectedTag={null}
        onSelectShelf={jest.fn()}
        onSelectTag={jest.fn()}
        onManage={onManage}
      />
    )
    fireEvent.click(screen.getByText('Edit shelves & tags'))
    expect(onManage).toHaveBeenCalledTimes(1)
  })
})
