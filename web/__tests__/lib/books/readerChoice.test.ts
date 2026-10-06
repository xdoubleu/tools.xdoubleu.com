import {
  loadReaderChoice,
  readerChoiceKey,
  readerFileFormat,
  requestedReaderChoice,
  saveReaderChoice
} from '@/lib/books/readerChoice'

describe('reader choice storage', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => jest.restoreAllMocks())

  it('keys the choice by book id', () => {
    expect(readerChoiceKey('book-1')).toBe('books:reader-choice:book-1')
  })

  it('round-trips a choice per book', () => {
    saveReaderChoice('book-1', 'kepub')
    expect(localStorage.getItem('books:reader-choice:book-1')).toBe('kepub')
    expect(loadReaderChoice('book-1')).toBe('kepub')
    expect(loadReaderChoice('book-2')).toBeNull()
    saveReaderChoice('book-1', 'original')
    expect(loadReaderChoice('book-1')).toBe('original')
  })

  it('ignores an unknown stored value', () => {
    localStorage.setItem('books:reader-choice:book-1', 'mobi')
    expect(loadReaderChoice('book-1')).toBeNull()
  })

  it('survives storage that throws', () => {
    jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(() => saveReaderChoice('book-1', 'kepub')).not.toThrow()
    expect(loadReaderChoice('book-1')).toBeNull()
  })
})

describe('requestedReaderChoice', () => {
  it('maps ?format= to a choice', () => {
    expect(requestedReaderChoice('kepub')).toBe('kepub')
    expect(requestedReaderChoice('epub')).toBe('original')
    expect(requestedReaderChoice('pdf')).toBe('original')
    expect(requestedReaderChoice('mobi')).toBeNull()
    expect(requestedReaderChoice(null)).toBeNull()
  })
})

describe('readerFileFormat', () => {
  it('opens the original by default', () => {
    expect(readerFileFormat('epub', 'original')).toBe('epub')
    expect(readerFileFormat('pdf', 'original')).toBe('pdf')
  })

  it('opens the KEPUB when converted is chosen', () => {
    expect(readerFileFormat('epub', 'kepub')).toBe('kepub')
    expect(readerFileFormat('pdf', 'kepub')).toBe('kepub')
  })
})
