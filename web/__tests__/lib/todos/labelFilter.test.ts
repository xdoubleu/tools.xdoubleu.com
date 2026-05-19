import { filterLabels, matchesLabel, normalizeLabels } from '@/lib/todos/labelFilter'

const PRESETS = ['bug', 'feature', 'enhancement', 'documentation', 'Frontend', 'backend']

describe('filterLabels', () => {
  it('returns all presets when query is empty string', () => {
    expect(filterLabels(PRESETS, '')).toEqual(PRESETS)
  })

  it('returns all presets when query is whitespace only', () => {
    expect(filterLabels(PRESETS, '   ')).toEqual(PRESETS)
  })

  it('returns exact match', () => {
    expect(filterLabels(PRESETS, 'bug')).toEqual(['bug'])
  })

  it('returns partial match', () => {
    const result = filterLabels(PRESETS, 'end')
    expect(result).toContain('backend')
    expect(result).toContain('Frontend')
  })

  it('is case-insensitive', () => {
    expect(filterLabels(PRESETS, 'FRONT')).toContain('Frontend')
    expect(filterLabels(PRESETS, 'front')).toContain('Frontend')
  })

  it('returns empty array when no presets match', () => {
    expect(filterLabels(PRESETS, 'zzz')).toEqual([])
  })

  it('matches multiple presets with shared substring', () => {
    const result = filterLabels(PRESETS, 'e')
    // "feature", "enhancement", "documentation", "Frontend", "backend" all contain 'e'
    expect(result.length).toBeGreaterThan(1)
  })

  it('trims leading/trailing whitespace from query', () => {
    expect(filterLabels(PRESETS, '  bug  ')).toEqual(['bug'])
  })
})

describe('matchesLabel', () => {
  it('returns true for exact match', () => {
    expect(matchesLabel('bug', 'bug')).toBe(true)
  })

  it('returns true for partial match', () => {
    expect(matchesLabel('feature', 'feat')).toBe(true)
  })

  it('is case-insensitive', () => {
    expect(matchesLabel('Frontend', 'front')).toBe(true)
    expect(matchesLabel('frontend', 'FRONT')).toBe(true)
  })

  it('returns false when query does not match label', () => {
    expect(matchesLabel('bug', 'feature')).toBe(false)
  })

  it('returns true when query is empty string (every label matches)', () => {
    expect(matchesLabel('anything', '')).toBe(true)
  })
})

describe('normalizeLabels', () => {
  it('splits comma-separated string into array', () => {
    expect(normalizeLabels('label1, label2')).toEqual(['label1', 'label2'])
  })

  it('trims whitespace around each label', () => {
    expect(normalizeLabels('  foo  ,  bar  ')).toEqual(['foo', 'bar'])
  })

  it('filters out empty segments from extra commas', () => {
    expect(normalizeLabels('a,,b')).toEqual(['a', 'b'])
  })

  it('returns empty array for empty string', () => {
    expect(normalizeLabels('')).toEqual([])
  })

  it('returns single-element array for single label', () => {
    expect(normalizeLabels('only')).toEqual(['only'])
  })

  it('handles trailing comma', () => {
    expect(normalizeLabels('x, y,')).toEqual(['x', 'y'])
  })

  it('handles comma-only string', () => {
    expect(normalizeLabels(',')).toEqual([])
  })
})
