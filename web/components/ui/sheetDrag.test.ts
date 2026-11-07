import { describe, expect, it } from 'vitest'
import { dragVelocity, shouldDismissSheet } from './sheetDrag'

describe('shouldDismissSheet', () => {
  it('ignores upward or zero offsets even when fast', () => {
    expect(shouldDismissSheet(0, 2, 400)).toBe(false)
    expect(shouldDismissSheet(-30, 2, 400)).toBe(false)
  })
  it('dismisses on a downward fling', () => {
    expect(shouldDismissSheet(20, 0.6, 400)).toBe(true)
  })
  it('dismisses past the distance threshold', () => {
    expect(shouldDismissSheet(160, 0.1, 400)).toBe(true)
  })
  it('snaps back on a short slow pull', () => {
    expect(shouldDismissSheet(100, 0.1, 400)).toBe(false)
  })
  it('does not divide by a zero height', () => {
    expect(shouldDismissSheet(100, 0.1, 0)).toBe(false)
  })
})

describe('dragVelocity', () => {
  it('is px per ms', () => {
    expect(dragVelocity(50, 100)).toBe(0.5)
  })
  it('is 0 without elapsed time', () => {
    expect(dragVelocity(50, 0)).toBe(0)
  })
})
