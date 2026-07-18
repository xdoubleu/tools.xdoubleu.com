import {
  SPECIAL_TAGS,
  BUILT_IN_STATUSES,
  BOOK_STATUSES,
  statusLabel,
  isBuiltInShelfId,
  displayTags
} from '@/lib/reading/bookShelves'

describe('SPECIAL_TAGS', () => {
  it('includes all reserved tags', () => {
    expect(SPECIAL_TAGS.has('favourite')).toBe(true)
    expect(SPECIAL_TAGS.has('own-physical')).toBe(true)
    expect(SPECIAL_TAGS.has('own-digital')).toBe(true)
    expect(SPECIAL_TAGS.has('kobo-sync')).toBe(true)
    expect(SPECIAL_TAGS.has('kobo-format-pdf')).toBe(true)
  })

  it('does not include user-visible tags', () => {
    expect(SPECIAL_TAGS.has('fantasy')).toBe(false)
    expect(SPECIAL_TAGS.has('sci-fi')).toBe(false)
  })
})

describe('BUILT_IN_STATUSES', () => {
  it('includes the four reading states', () => {
    expect(BUILT_IN_STATUSES.has('to-read')).toBe(true)
    expect(BUILT_IN_STATUSES.has('currently-reading')).toBe(true)
    expect(BUILT_IN_STATUSES.has('read')).toBe(true)
    expect(BUILT_IN_STATUSES.has('dropped')).toBe(true)
  })

  it('does not include custom shelf names', () => {
    expect(BUILT_IN_STATUSES.has('my-shelf')).toBe(false)
  })
})

describe('BOOK_STATUSES', () => {
  it('has exactly four entries', () => {
    expect(BOOK_STATUSES).toHaveLength(4)
  })

  it('each entry has a value and label', () => {
    for (const s of BOOK_STATUSES) {
      expect(typeof s.value).toBe('string')
      expect(typeof s.label).toBe('string')
    }
  })
})

describe('statusLabel', () => {
  it('returns a friendly label for built-in statuses', () => {
    expect(statusLabel('to-read')).toBe('Want to read')
    expect(statusLabel('currently-reading')).toBe('Currently reading')
    expect(statusLabel('read')).toBe('Read')
    expect(statusLabel('dropped')).toBe('Dropped')
  })

  it('returns the raw value for unknown / custom statuses', () => {
    expect(statusLabel('my-custom-shelf')).toBe('my-custom-shelf')
  })
})

describe('isBuiltInShelfId', () => {
  it('treats the four reading statuses and favourite as built-in', () => {
    expect(isBuiltInShelfId('to-read')).toBe(true)
    expect(isBuiltInShelfId('currently-reading')).toBe(true)
    expect(isBuiltInShelfId('read')).toBe(true)
    expect(isBuiltInShelfId('dropped')).toBe(true)
    expect(isBuiltInShelfId('favourite')).toBe(true)
  })

  it('treats custom shelf names as not built-in', () => {
    expect(isBuiltInShelfId('my-shelf')).toBe(false)
  })
})

describe('displayTags', () => {
  it('filters out all special tags', () => {
    const input = ['favourite', 'own-physical', 'own-digital', 'fantasy', 'sci-fi']
    expect(displayTags(input)).toEqual(['fantasy', 'sci-fi'])
  })

  it('returns an empty array when all tags are special', () => {
    expect(displayTags(['favourite', 'kobo-sync'])).toEqual([])
  })

  it('returns all tags when none are special', () => {
    const input = ['fantasy', '2024']
    expect(displayTags(input)).toEqual(input)
  })
})
