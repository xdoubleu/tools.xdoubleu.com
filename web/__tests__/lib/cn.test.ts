import { cn } from '@/lib/cn'

describe('cn', () => {
  it('joins class names', () => {
    expect(cn('a', 'b')).toBe('a b')
  })

  it('skips falsy values', () => {
    expect(cn('a', false, null, undefined, 'b')).toBe('a b')
  })

  it('supports conditional object syntax', () => {
    expect(cn('a', { b: true, c: false })).toBe('a b')
  })

  it('resolves conflicting tailwind utilities, keeping the last one', () => {
    expect(cn('w-full', 'w-16')).toBe('w-16')
    expect(cn('rounded-full', 'rounded-lg')).toBe('rounded-lg')
    expect(cn('px-4', 'px-0')).toBe('px-0')
  })
})
