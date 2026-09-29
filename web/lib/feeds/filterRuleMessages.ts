import { ConnectError, Code } from '@connectrpc/connect'

// filteredMessage reports how many existing items a new rule filtered.
export function filteredMessage(count: number): string {
  if (count === 0) return 'Rule added. No existing items matched.'
  return `Filtered ${count} existing ${count === 1 ? 'item' : 'items'}.`
}

export function createRuleErrorMessage(err: unknown): string {
  if (err instanceof ConnectError && err.code === Code.AlreadyExists) {
    return 'That rule already exists.'
  }
  return 'Adding the rule failed. Please try again.'
}
