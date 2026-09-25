/**
 * runPool runs up to `limit` concurrent `worker` calls over `items`, rejecting
 * like Promise.all if any throws. The shared index is claimed synchronously
 * before any `await`, so there's no race.
 */
export async function runPool<T>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<void>
): Promise<void> {
  let index = 0

  async function drain(): Promise<void> {
    while (index < items.length) {
      const item = items[index++]
      await worker(item)
    }
  }

  const workerCount = Math.min(limit, items.length)
  if (workerCount === 0) return

  await Promise.all(Array.from({ length: workerCount }, drain))
}
