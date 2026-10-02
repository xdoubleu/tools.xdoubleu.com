import { isMessage } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  FeedService,
  GetFeedItemResponseSchema,
  GetFilterRuleSuggestionsResponseSchema,
  ListFeedItemsResponseSchema,
  ListFeedsResponseSchema,
  ListFilterRulesResponseSchema,
  type Item,
  type UpdateItemRequest
} from '@/lib/gen/feeds/v1/feeds_pb'

const { method } = FeedService
const REVALIDATE = '/feeds'
const FILTERED = swrKeys.feedFilteredItems().split('?')[0]

/** Applies UpdateItem's set fields; `readAt` stamps an item newly read. */
function patchItem(item: Item, req: UpdateItemRequest, readAt: string): Item {
  let next = item.readAt
  if (req.read !== undefined) next = req.read ? item.readAt || readAt : ''
  const pct = req.readProgressPct
  return {
    ...item,
    readAt: next,
    dismissed: req.dismissed ?? item.dismissed,
    bookmarked: req.bookmarked ?? item.bookmarked,
    readProgressPct:
      pct === undefined ? item.readProgressPct : Math.max(item.readProgressPct, Math.min(pct, 100))
  }
}

/** Runs `fn` on the items of any cached item list (inbox, filtered, summary). */
function onItemLists(key: unknown, data: unknown, fn: (items: Item[]) => Item[]): unknown {
  if (typeof key !== 'string' || !key.startsWith(REVALIDATE)) return data
  return isMessage(data, ListFeedItemsResponseSchema) ? { ...data, items: fn(data.items) } : data
}

export const updateItemWrite = defineOfflineWrite({
  method: method.updateItem,
  apply: (key, data, req, hint) => {
    const readAt = typeof hint === 'string' ? hint : ''
    if (key === swrKeys.feedItem(req.itemId) && isMessage(data, GetFeedItemResponseSchema)) {
      return data.item ? { ...data, item: patchItem(data.item, req, readAt) } : data
    }
    return onItemLists(key, data, (items) =>
      items.map((i) => (i.id === req.itemId ? patchItem(i, req, readAt) : i))
    )
  },
  describe: () => 'Update an article',
  revalidate: REVALIDATE,
  revalidateOnSuccess: false
})

export const restoreFeedItemWrite = defineOfflineWrite({
  method: method.restoreFeedItem,
  apply: (key, data, req) =>
    typeof key === 'string' && key.startsWith(FILTERED)
      ? onItemLists(key, data, (items) => items.filter((i) => i.id !== req.itemId))
      : data,
  describe: () => 'Restore a filtered article',
  revalidate: REVALIDATE
})

export const deleteFeedWrite = defineOfflineWrite({
  method: method.deleteFeed,
  apply: (key, data, req) => {
    const other = <T extends { feedId: string }>(list: T[]) =>
      list.filter((e) => e.feedId !== req.feedId)
    if (key === swrKeys.feeds && isMessage(data, ListFeedsResponseSchema)) {
      return { ...data, feeds: data.feeds.filter((f) => f.id !== req.feedId) }
    }
    if (key === swrKeys.feedFilterRules && isMessage(data, ListFilterRulesResponseSchema)) {
      return { ...data, rules: other(data.rules) }
    }
    if (
      key === swrKeys.feedFilterRuleSuggestions &&
      isMessage(data, GetFilterRuleSuggestionsResponseSchema)
    ) {
      return { ...data, suggestions: other(data.suggestions) }
    }
    return onItemLists(key, data, other)
  },
  describe: () => 'Remove a feed',
  revalidate: REVALIDATE
})

export const deleteFilterRuleWrite = defineOfflineWrite({
  method: method.deleteFilterRule,
  apply: (key, data, req) =>
    key === swrKeys.feedFilterRules && isMessage(data, ListFilterRulesResponseSchema)
      ? { ...data, rules: data.rules.filter((r) => r.id !== req.ruleId) }
      : data,
  describe: () => 'Delete a filter rule',
  revalidate: REVALIDATE
})

export const dismissSuggestionWrite = defineOfflineWrite({
  method: method.dismissFilterRuleSuggestion,
  apply: (key, data, req) =>
    key === swrKeys.feedFilterRuleSuggestions &&
    isMessage(data, GetFilterRuleSuggestionsResponseSchema)
      ? {
          ...data,
          suggestions: data.suggestions.filter(
            (s) => s.feedId !== req.feedId || s.category !== req.category
          )
        }
      : data,
  describe: (req) => `Dismiss the “${req.category}” suggestion`,
  revalidate: REVALIDATE
})

export const feedWrites = [
  updateItemWrite,
  restoreFeedItemWrite,
  deleteFeedWrite,
  deleteFilterRuleWrite,
  dismissSuggestionWrite
]
