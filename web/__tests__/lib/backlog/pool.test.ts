import { runPool } from '@/lib/backlog/pool'

describe('runPool', () => {
  it('processes all items', async () => {
    const results: number[] = []
    await runPool([1, 2, 3, 4, 5], 2, async (n) => {
      results.push(n)
    })
    expect(results).toHaveLength(5)
    expect(results.sort((a, b) => a - b)).toEqual([1, 2, 3, 4, 5])
  })

  it('resolves immediately for an empty list', async () => {
    const worker = jest.fn()
    await runPool([], 4, worker)
    expect(worker).not.toHaveBeenCalled()
  })

  it('respects the concurrency limit', async () => {
    let concurrent = 0
    let maxConcurrent = 0
    const limit = 3
    const items = Array.from({ length: 10 }, (_, i) => i)

    await runPool(items, limit, async () => {
      concurrent++
      maxConcurrent = Math.max(maxConcurrent, concurrent)
      // Yield to allow other workers to start before this one finishes.
      await Promise.resolve()
      concurrent--
    })

    expect(maxConcurrent).toBeLessThanOrEqual(limit)
  })

  it('still starts only as many workers as items when limit > items.length', async () => {
    const calls: number[] = []
    await runPool([42], 10, async (n) => {
      calls.push(n)
    })
    expect(calls).toEqual([42])
  })

  it('propagates a worker error', async () => {
    const boom = new Error('worker failed')
    await expect(
      runPool([1, 2, 3], 2, async (n) => {
        if (n === 2) throw boom
      })
    ).rejects.toThrow(boom)
  })
})
