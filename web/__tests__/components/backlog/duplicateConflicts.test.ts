import {
  detectConflicts,
  buildResolvedMetadata,
  pickAutoStatusBookId,
  resolveStatusChoice,
  type BookConflictField,
  type DupGroup
} from '@/components/backlog/duplicateConflicts'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeBook(
  overrides: Partial<{
    id: string
    title: string
    authors: string[]
    isbn13: string
    coverUrl: string
    description: string
    pageCount: number
  }> = {}
) {
  return {
    id: overrides.id ?? 'book-id',
    title: overrides.title ?? 'Some Title',
    authors: overrides.authors ?? ['Author'],
    isbn13: overrides.isbn13 ?? '',
    coverUrl: overrides.coverUrl ?? '',
    description: overrides.description ?? '',
    pageCount: overrides.pageCount ?? 0
  }
}

function makeEntry(bookId: string, book: ReturnType<typeof makeBook>, status = 'to-read') {
  return {
    id: `ub-${bookId}`,
    bookId,
    userId: 'user1',
    book,
    status,
    tags: [],
    rating: 0,
    finishedAt: [],
    addedAt: '',
    updatedAt: '',
    progressMode: 'pages',
    currentPage: 0,
    progressPercent: 0,
    formats: []
  }
}

function makeGroup(entries: ReturnType<typeof makeEntry>[], reason = 'isbn13'): DupGroup {
  return { entries, reason }
}

// ---------------------------------------------------------------------------
// detectConflicts
// ---------------------------------------------------------------------------

describe('detectConflicts', () => {
  it('returns empty when all fields agree', () => {
    const book = makeBook({ title: 'Same', pageCount: 300 })
    const g = makeGroup([
      makeEntry('a', { ...book, id: 'a' }),
      makeEntry('b', { ...book, id: 'b' })
    ])
    expect(detectConflicts(g)).toHaveLength(0)
  })

  it('detects a page count conflict', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', title: 'T', pageCount: 320 })),
      makeEntry('b', makeBook({ id: 'b', title: 'T', pageCount: 310 }))
    ])
    const conflicts = detectConflicts(g)
    const fields = conflicts.map((c) => c.field)
    expect(fields).toContain('pageCount')
  })

  it('detects a title conflict', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', title: 'Title A' })),
      makeEntry('b', makeBook({ id: 'b', title: 'Title B' }))
    ])
    const fields = detectConflicts(g).map((c) => c.field)
    expect(fields).toContain('title')
  })

  it('detects a cover conflict when presence differs', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', coverUrl: 'https://example.com/img.jpg' })),
      makeEntry('b', makeBook({ id: 'b', coverUrl: '' }))
    ])
    const fields = detectConflicts(g).map((c) => c.field)
    expect(fields).toContain('cover')
  })

  it('does not flag cover when both entries have a cover (presence matches)', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', coverUrl: 'https://example.com/a.jpg' })),
      makeEntry('b', makeBook({ id: 'b', coverUrl: 'https://example.com/b.jpg' }))
    ])
    const fields = detectConflicts(g).map((c) => c.field)
    expect(fields).not.toContain('cover')
  })

  it('returns choices with one entry per conflict value', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', pageCount: 300 })),
      makeEntry('b', makeBook({ id: 'b', pageCount: 400 }))
    ])
    const conflict = detectConflicts(g).find((c) => c.field === 'pageCount')
    expect(conflict).toBeDefined()
    expect(conflict!.choices).toHaveLength(2)
    expect(conflict!.choices[0].bookId).toBe('a')
    expect(conflict!.choices[1].bookId).toBe('b')
  })

  it('marks empty values as not having a value', () => {
    const g = makeGroup([
      makeEntry('a', makeBook({ id: 'a', isbn13: '9781234567890' })),
      makeEntry('b', makeBook({ id: 'b', isbn13: '' }))
    ])
    const conflict = detectConflicts(g).find((c) => c.field === 'isbn13')
    expect(conflict).toBeDefined()
    expect(conflict!.choices[0].hasValue).toBe(true)
    expect(conflict!.choices[1].hasValue).toBe(false)
  })

  it('detects a status conflict when entries have different statuses', () => {
    const book = makeBook({ id: 'x', title: 'Same' })
    const g = makeGroup([
      makeEntry('a', { ...book }, 'sci-fi'),
      makeEntry('b', { ...book }, 'read')
    ])
    const fields = detectConflicts(g).map((c) => c.field)
    expect(fields).toContain('status')
  })

  it('does not flag status conflict when both entries have the same status', () => {
    const book = makeBook({ id: 'x', title: 'Same' })
    const g = makeGroup([
      makeEntry('a', { ...book }, 'to-read'),
      makeEntry('b', { ...book }, 'to-read')
    ])
    const fields = detectConflicts(g).map((c) => c.field)
    expect(fields).not.toContain('status')
  })

  it('status conflict choices carry the entry status as displayValue', () => {
    const book = makeBook({ id: 'x', title: 'Same' })
    const g = makeGroup([
      makeEntry('a', { ...book }, 'sci-fi'),
      makeEntry('b', { ...book }, 'read')
    ])
    const conflict = detectConflicts(g).find((c) => c.field === 'status')
    expect(conflict).toBeDefined()
    expect(conflict!.choices[0].displayValue).toBe('sci-fi')
    expect(conflict!.choices[1].displayValue).toBe('read')
  })
})

// ---------------------------------------------------------------------------
// pickAutoStatusBookId
// ---------------------------------------------------------------------------

describe('pickAutoStatusBookId', () => {
  it('prefers a custom shelf over built-in read status', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('shelf-id', { ...book }, 'sci-fi'),
      makeEntry('read-id', { ...book }, 'read')
    ])
    expect(pickAutoStatusBookId(g)).toBe('shelf-id')
  })

  it('prefers a custom shelf even when winner entry has built-in status', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('read-id', { ...book }, 'read'),
      makeEntry('shelf-id', { ...book }, 'favourites')
    ])
    expect(pickAutoStatusBookId(g)).toBe('shelf-id')
  })

  it('prefers read over currently-reading', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('reading-id', { ...book }, 'currently-reading'),
      makeEntry('read-id', { ...book }, 'read')
    ])
    expect(pickAutoStatusBookId(g)).toBe('read-id')
  })

  it('prefers currently-reading over to-read', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('toread-id', { ...book }, 'to-read'),
      makeEntry('reading-id', { ...book }, 'currently-reading')
    ])
    expect(pickAutoStatusBookId(g)).toBe('reading-id')
  })

  it('falls back to entries[0] when both are on custom shelves (tie)', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('first-id', { ...book }, 'sci-fi'),
      makeEntry('second-id', { ...book }, 'fantasy')
    ])
    expect(pickAutoStatusBookId(g)).toBe('first-id')
  })

  it('dropped loses to to-read', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('dropped-id', { ...book }, 'dropped'),
      makeEntry('toread-id', { ...book }, 'to-read')
    ])
    expect(pickAutoStatusBookId(g)).toBe('toread-id')
  })
})

// ---------------------------------------------------------------------------
// resolveStatusChoice
// ---------------------------------------------------------------------------

describe('resolveStatusChoice', () => {
  it('returns the status of the chosen entry', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([
      makeEntry('a', { ...book }, 'sci-fi'),
      makeEntry('b', { ...book }, 'read')
    ])
    const choices: Partial<Record<BookConflictField, string>> = { status: 'b' }
    expect(resolveStatusChoice(g, choices)).toBe('read')
  })

  it('returns undefined when status field is not in choices', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([makeEntry('a', { ...book }, 'sci-fi')])
    const choices: Partial<Record<BookConflictField, string>> = {}
    expect(resolveStatusChoice(g, choices)).toBeUndefined()
  })

  it('returns undefined when the chosen bookId is not found in entries', () => {
    const book = makeBook({ id: 'x' })
    const g = makeGroup([makeEntry('a', { ...book }, 'sci-fi')])
    const choices: Partial<Record<BookConflictField, string>> = { status: 'nonexistent' }
    expect(resolveStatusChoice(g, choices)).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// buildResolvedMetadata
// ---------------------------------------------------------------------------

describe('buildResolvedMetadata', () => {
  it('returns winner fields when all choices point to winner', () => {
    const winnerBook = makeBook({ id: 'w', title: 'Winner Title', pageCount: 300 })
    const loserBook = makeBook({ id: 'l', title: 'Loser Title', pageCount: 200 })
    const g = makeGroup([makeEntry('w', winnerBook), makeEntry('l', loserBook)])
    const choices: Record<BookConflictField, string> = {
      status: 'w',
      title: 'w',
      authors: 'w',
      isbn13: 'w',
      cover: 'w',
      description: 'w',
      pageCount: 'w'
    }
    const result = buildResolvedMetadata(g, choices)
    expect(result.title).toBe('Winner Title')
    expect(result.pageCount).toBe(300)
  })

  it('overrides chosen fields from loser', () => {
    const winnerBook = makeBook({ id: 'w', title: 'Winner Title', pageCount: 300 })
    const loserBook = makeBook({ id: 'l', title: 'Loser Title', pageCount: 400 })
    const g = makeGroup([makeEntry('w', winnerBook), makeEntry('l', loserBook)])
    const choices: Record<BookConflictField, string> = {
      status: 'w',
      title: 'w',
      authors: 'w',
      isbn13: 'w',
      cover: 'w',
      description: 'w',
      pageCount: 'l' // pick loser's page count
    }
    const result = buildResolvedMetadata(g, choices)
    expect(result.title).toBe('Winner Title')
    expect(result.pageCount).toBe(400)
  })

  it('excludes coverUrl from resolved metadata', () => {
    const winnerBook = makeBook({ id: 'w', coverUrl: 'https://example.com/w.jpg' })
    const loserBook = makeBook({ id: 'l', coverUrl: 'https://example.com/l.jpg' })
    const g = makeGroup([makeEntry('w', winnerBook), makeEntry('l', loserBook)])
    const choices: Record<BookConflictField, string> = {
      status: 'w',
      title: 'w',
      authors: 'w',
      isbn13: 'w',
      cover: 'l',
      description: 'w',
      pageCount: 'w'
    }
    const result = buildResolvedMetadata(g, choices)
    // coverUrl must be empty in resolved metadata (proto object always has the
    // field, but we must not copy any source book's URL into it)
    expect(result.coverUrl).toBe('')
  })

  it('excludes status from resolved metadata (status is a UserBook field, not catalog)', () => {
    const winnerBook = makeBook({ id: 'w', title: 'Title' })
    const loserBook = makeBook({ id: 'l', title: 'Title' })
    const g = makeGroup([
      makeEntry('w', winnerBook, 'to-read'),
      makeEntry('l', loserBook, 'sci-fi')
    ])
    const choices: Record<BookConflictField, string> = {
      status: 'l', // pick loser's shelf
      title: 'w',
      authors: 'w',
      isbn13: 'w',
      cover: 'w',
      description: 'w',
      pageCount: 'w'
    }
    const result = buildResolvedMetadata(g, choices)
    // The catalog Book proto has no status field; the choice must not bleed in.
    expect('status' in result).toBe(false)
  })
})
