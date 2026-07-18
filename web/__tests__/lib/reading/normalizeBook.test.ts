import { normalizeTitle, normalizeAuthor, isbnLessGroupKey } from '@/lib/reading/normalizeBook'

describe('normalizeTitle', () => {
  it('strips subtitle (everything after colon)', () => {
    expect(normalizeTitle('Dune: The Novel')).toBe('dune')
  })

  it('lowercases and removes non-alphanumeric characters', () => {
    expect(normalizeTitle("Harry Potter's")).toBe('harrypotters')
  })

  it('strips diacritics', () => {
    expect(normalizeTitle('Café au lait')).toBe('cafeaulait')
  })

  it('returns empty string for an empty input', () => {
    expect(normalizeTitle('')).toBe('')
  })

  it('keeps digits', () => {
    expect(normalizeTitle('2001: A Space Odyssey')).toBe('2001')
  })

  it('drops a leading article', () => {
    expect(normalizeTitle('The Hobbit')).toBe('hobbit')
  })

  it('keeps a single-word title equal to an article intact', () => {
    expect(normalizeTitle('A')).toBe('a')
  })

  it('strips a parenthetical series annotation', () => {
    expect(normalizeTitle("Firekeeper's Daughter (Firekeeper's Daughter, #1)")).toBe(
      normalizeTitle("Firekeeper's Daughter")
    )
  })

  it('strips a bracketed annotation', () => {
    expect(normalizeTitle('Dune [Illustrated]')).toBe(normalizeTitle('Dune'))
  })

  it('strips a trailing edition marker after " - "', () => {
    expect(normalizeTitle('Dune - Deluxe Edition')).toBe(normalizeTitle('Dune'))
  })

  it('distinguishes a volume number after a colon', () => {
    expect(normalizeTitle('System Design Interview: Volume 1')).not.toBe(
      normalizeTitle('System Design Interview: Volume 2')
    )
  })

  it('distinguishes a volume number after " - "', () => {
    expect(normalizeTitle('System Design Interview - Volume 1')).not.toBe(
      normalizeTitle('System Design Interview - Volume 2')
    )
  })

  it('distinguishes a volume number in a parenthetical', () => {
    expect(normalizeTitle('System Design Interview (Volume 1)')).not.toBe(
      normalizeTitle('System Design Interview (Volume 2)')
    )
  })

  it('does not double-count a number already in the retained segment', () => {
    expect(normalizeTitle('2001: A Space Odyssey')).toBe('2001')
  })

  it('does not duplicate a volume number that already precedes the colon', () => {
    expect(normalizeTitle('Mistborn Book 2: Legendary Heroes')).toBe(
      normalizeTitle('Mistborn Book 2')
    )
  })
})

describe('normalizeAuthor', () => {
  it('handles "Last, First" format (comma present)', () => {
    expect(normalizeAuthor('Tolkien, J.R.R.')).toBe('tolkien')
  })

  it('handles "First Last" format (no comma)', () => {
    expect(normalizeAuthor('J.R.R. Tolkien')).toBe('tolkien')
  })

  it('returns empty string for empty input', () => {
    expect(normalizeAuthor('')).toBe('')
  })

  it('returns empty string for whitespace-only input', () => {
    expect(normalizeAuthor('   ')).toBe('')
  })

  it('strips diacritics from author last name', () => {
    expect(normalizeAuthor('José Saramago')).toBe('saramago')
  })

  it('handles single-word author name', () => {
    expect(normalizeAuthor('Homer')).toBe('homer')
  })
})

describe('isbnLessGroupKey', () => {
  it('returns a stable key for a given title and author', () => {
    const key1 = isbnLessGroupKey('Dune', ['Frank Herbert'])
    const key2 = isbnLessGroupKey('Dune', ['Frank Herbert'])
    expect(key1).toBe(key2)
    expect(key1).not.toBeNull()
  })

  it('treats subtitle-differing titles as the same', () => {
    const key1 = isbnLessGroupKey('Dune', ['Frank Herbert'])
    const key2 = isbnLessGroupKey('Dune: The Original', ['Frank Herbert'])
    expect(key1).toBe(key2)
  })

  it('treats books with the same last name as matching', () => {
    const key1 = isbnLessGroupKey('Dune', ['Frank Herbert'])
    const key2 = isbnLessGroupKey('Dune', ['Brian Herbert'])
    // Different first names but same last name — should produce the same key
    // (mirrors the Go normalizeAuthor last-name-only logic).
    expect(key1).toBe(key2)
  })

  it('returns null when title normalizes to empty', () => {
    expect(isbnLessGroupKey('', ['Author'])).toBeNull()
    expect(isbnLessGroupKey('---', ['Author'])).toBeNull()
  })

  it('returns null when authors list is empty', () => {
    expect(isbnLessGroupKey('Dune', [])).toBeNull()
  })

  it('returns null when first author normalizes to empty', () => {
    expect(isbnLessGroupKey('Dune', ['---'])).toBeNull()
  })

  it('uses only the first author for the key', () => {
    const key1 = isbnLessGroupKey('Dune', ['Frank Herbert', 'Another Author'])
    const key2 = isbnLessGroupKey('Dune', ['Frank Herbert'])
    expect(key1).toBe(key2)
  })

  it('treats different volumes by the same author as distinct', () => {
    const key1 = isbnLessGroupKey('System Design Interview: Volume 1', ['Alex Xu'])
    const key2 = isbnLessGroupKey('System Design Interview: Volume 2', ['Alex Xu'])
    expect(key1).not.toBe(key2)
  })
})
