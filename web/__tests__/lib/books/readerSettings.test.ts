import {
  READER_THEMES,
  READER_FONT_SIZE_DEFAULT,
  READER_FONT_SIZE_MAX,
  READER_FONT_SIZE_MIN,
  fixedLayoutFilter,
  pickReaderFormat,
  readerBackground,
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

describe('READER_THEMES', () => {
  it('lists the themes in display order', () => {
    expect(READER_THEMES).toEqual([
      { value: 'light', label: 'Light' },
      { value: 'sepia', label: 'Sepia' },
      { value: 'dark', label: 'Dark' }
    ])
  })
})

describe('readerBackground', () => {
  it('matches the page colour of each theme', () => {
    expect(readerBackground('light')).toBe('#ffffff')
    expect(readerBackground('sepia')).toBe('#f4ecd8')
    expect(readerBackground('dark')).toBe('#161616')
  })
})

describe('readerStyles', () => {
  it.each([
    ['light', '#ffffff', '#1f1f1f', '#1d4ed8'],
    ['sepia', '#f4ecd8', '#5b4636', '#8a4b14'],
    ['dark', '#161616', '#e3e3e3', '#8ab4f8']
  ] as const)('colours %s pages', (theme, bg, fg, link) => {
    const css = readerStyles(theme, 100)
    expect(css).toContain(`background: ${bg} !important`)
    expect(css).toContain(`color: ${fg} !important`)
    expect(css).toContain(`color: ${link} !important`)
  })

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

  it('includes both edges of the area', () => {
    expect(tapDirection(0, 0, 100)).toBe('left')
    expect(tapDirection(100, 0, 100)).toBe('right')
  })

  it('leaves the band between the edge zones alone', () => {
    expect(tapDirection(30, 0, 100)).toBeNull()
    expect(tapDirection(70, 0, 100)).toBeNull()
  })

  it('ignores taps outside the area', () => {
    expect(tapDirection(150, 0, 100)).toBeNull()
    expect(tapDirection(-10, 0, 100)).toBeNull()
  })

  it('is relative to the reading area', () => {
    expect(tapDirection(110, 100, 100)).toBe('left')
    expect(tapDirection(50, 100, 100)).toBeNull()
  })

  it('ignores an empty area', () => {
    expect(tapDirection(0, 0, 0)).toBeNull()
    expect(tapDirection(5, 0, 0)).toBeNull()
    expect(tapDirection(-5, 0, 0)).toBeNull()
  })
})

describe('swipeDirection', () => {
  it('maps a leftward swipe to the next page on the right', () => {
    expect(swipeDirection(-80, 5)).toBe('right')
    expect(swipeDirection(80, 5)).toBe('left')
  })

  it('turns from the minimum swipe distance', () => {
    expect(swipeDirection(-40, 0)).toBe('right')
    expect(swipeDirection(40, 0)).toBe('left')
  })

  it('turns on an exact diagonal', () => {
    expect(swipeDirection(-80, 80)).toBe('right')
  })

  it('ignores short or vertical swipes', () => {
    expect(swipeDirection(-20, 0)).toBeNull()
    expect(swipeDirection(39, 0)).toBeNull()
    expect(swipeDirection(-80, 120)).toBeNull()
  })
})
