import { categoryLabel, categoryOf } from '@/lib/reading/categories'

describe('categories', () => {
  it('normalizes unknown or empty categories to book', () => {
    expect(categoryOf(undefined)).toBe('book')
    expect(categoryOf('')).toBe('book')
    expect(categoryOf('bogus')).toBe('book')
    expect(categoryOf('paper')).toBe('paper')
    expect(categoryOf('rss')).toBe('rss')
  })

  it('labels categories', () => {
    expect(categoryLabel('paper')).toBe('Paper')
    expect(categoryLabel('article')).toBe('Article')
    expect(categoryLabel(undefined)).toBe('Book')
  })
})
