import {
  READER_FONT_SIZE_DEFAULT,
  READER_FONT_SIZE_MAX,
  READER_FONT_SIZE_MIN,
  fixedLayoutFilter,
  pickReaderFormat,
  readerFormats,
  readerStyles,
  stepFontSize,
  swipeDirection,
  tapDirection
} from '@/lib/books/readerSettings'

describe('readerFormats', () => {
  it('lists the original formats in preference order', () => {
    expect(readerFormats(['pdf', 'kepub', 'epub'])).toEqual(['epub', 'pdf'])
  })

  it('ignores formats the reader does not open', () => {
    expect(readerFormats(['kepub'])).toEqual([])
  })
})

describe('pickReaderFormat', () => {
  it('prefers EPUB when the book has both', () => {
    expect(pickReaderFormat(['pdf', 'epub'], null)).toBe('epub')
  })

  it('falls back to PDF', () => {
    expect(pickReaderFormat(['pdf'], null)).toBe('pdf')
  })

  it('honours a requested format the book has', () => {
    expect(pickReaderFormat(['epub', 'pdf'], 'pdf')).toBe('pdf')
  })

  it('ignores a requested format the book lacks', () => {
    expect(pickReaderFormat(['epub'], 'pdf')).toBe('epub')
  })

  it('returns null without an EPUB or PDF', () => {
    expect(pickReaderFormat(['kepub'], 'epub')).toBeNull()
    expect(pickReaderFormat([], null)).toBeNull()
  })
})

describe('readerStyles', () => {
  it('applies the theme colours and font size', () => {
    const css = readerStyles('dark', 120)
    expect(css).toContain('color-scheme: dark')
    expect(css).toContain('font-size: 120%')
    expect(css).toMatch(/background: #[0-9a-f]{6} !important/)
  })

  it('uses distinct colours per theme', () => {
    expect(readerStyles('light', 100)).not.toEqual(readerStyles('sepia', 100))
    expect(readerStyles('sepia', 100)).toContain('color-scheme: light')
  })
})

describe('fixedLayoutFilter', () => {
  it('leaves light pages untouched', () => {
    expect(fixedLayoutFilter('light')).toBe('none')
  })

  it('inverts dark pages and tints sepia ones', () => {
    expect(fixedLayoutFilter('dark')).toContain('invert')
    expect(fixedLayoutFilter('sepia')).toContain('sepia')
  })
})

describe('stepFontSize', () => {
  it('steps by 10%', () => {
    expect(stepFontSize(READER_FONT_SIZE_DEFAULT, 1)).toBe(110)
    expect(stepFontSize(READER_FONT_SIZE_DEFAULT, -1)).toBe(90)
  })

  it('clamps to the supported range', () => {
    expect(stepFontSize(READER_FONT_SIZE_MAX, 1)).toBe(READER_FONT_SIZE_MAX)
    expect(stepFontSize(READER_FONT_SIZE_MIN, -1)).toBe(READER_FONT_SIZE_MIN)
  })
})

describe('tapDirection', () => {
  it('turns back on the left edge and forward on the right edge', () => {
    expect(tapDirection(10, 0, 100)).toBe('left')
    expect(tapDirection(90, 0, 100)).toBe('right')
  })

  it('does nothing in the middle', () => {
    expect(tapDirection(50, 0, 100)).toBeNull()
  })

  it('is relative to the reading area', () => {
    expect(tapDirection(110, 100, 100)).toBe('left')
    expect(tapDirection(50, 100, 100)).toBeNull()
  })

  it('ignores an empty area', () => {
    expect(tapDirection(0, 0, 0)).toBeNull()
  })
})

describe('swipeDirection', () => {
  it('maps a leftward swipe to the next page on the right', () => {
    expect(swipeDirection(-80, 5)).toBe('right')
    expect(swipeDirection(80, 5)).toBe('left')
  })

  it('ignores short or vertical swipes', () => {
    expect(swipeDirection(-20, 0)).toBeNull()
    expect(swipeDirection(-80, 120)).toBeNull()
  })
})
