/**
 * runPool runs up to `limit` concurrent calls to `worker` over `items`.
 *
 * Workers pull items from a shared index — each worker loops until the list is
 * exhausted, then resolves. The function resolves once every item has been
 * processed. Worker errors propagate: if any worker throws, the rejection is
 * forwarded from runPool (matching Promise.all behaviour).
 *
 * Safe in a single-threaded JS environment: the shared index is read and
 * incremented in the same synchronous step before any `await`, so there are
 * no data races.
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
