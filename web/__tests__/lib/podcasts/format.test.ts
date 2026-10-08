import { durationLabel } from '@/lib/podcasts/format'

describe('durationLabel', () => {
  it.each([
    [undefined, ''],
    [0, ''],
    [-5, ''],
    [20, '1 min'],
    [89, '1 min'],
    [90, '2 min'],
    [2700, '45 min'],
    [3540, '59 min'],
    [3600, '1h'],
    [3723, '1h 2m'],
    [7380, '2h 3m']
  ])('%s -> %j', (seconds, label) => {
    expect(durationLabel(seconds)).toBe(label)
  })
})
