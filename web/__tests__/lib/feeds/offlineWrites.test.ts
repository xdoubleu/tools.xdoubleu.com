import { create, isMessage, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetFeedItemResponseSchema,
  GetFilterRuleSuggestionsResponseSchema,
  ListFeedItemsResponseSchema,
  ListFeedsResponseSchema,
  ListFilterRulesResponseSchema
} from '@/lib/gen/feeds/v1/feeds_pb'
import {
  deleteFeedWrite,
  deleteFilterRuleWrite,
  dismissSuggestionWrite,
  feedWrites,
  restoreFeedItemWrite,
  updateItemWrite
} from '@/lib/feeds/offlineWrites'

function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  key: unknown,
  data: unknown,
  hint?: unknown
) {
  return write.apply(key, data, write.encode(init), hint)
}

const NOW = '2024-05-01T10:00:00.000Z'
const inbox = swrKeys.feedItems(true)

const items = () =>
  create(ListFeedItemsResponseSchema, {
    items: [
      { id: 'i1', feedId: 'f1', readProgressPct: 40 },
      { id: 'i2', feedId: 'f2', readAt: '2024-04-01T00:00:00Z', bookmarked: true }
    ],
    hasMore: true
  })

const listed = (data: unknown) => (isMessage(data, ListFeedItemsResponseSchema) ? data.items : [])

describe('feed offline writes', () => {
  it('leave unrelated keys and data untouched', () => {
    const data = items()
    for (const write of feedWrites) {
      expect(write.apply('/recipes', data, new Uint8Array(), undefined)).toBe(data)
      expect(write.apply(['/feeds/items'], data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(updateItemWrite, { itemId: 'i1', read: true }, inbox, 'nope', NOW)).toBe('nope')
  })

  it('marks an item read at the queued time, keeping an earlier read time', () => {
    const read = listed(run(updateItemWrite, { itemId: 'i1', read: true }, inbox, items(), NOW))
    expect(read[0]).toMatchObject({ id: 'i1', readAt: NOW, readProgressPct: 40 })
    expect(read[1]).toMatchObject({ id: 'i2', readAt: '2024-04-01T00:00:00Z' })

    const again = listed(run(updateItemWrite, { itemId: 'i2', read: true }, inbox, items(), NOW))
    expect(again[1].readAt).toBe('2024-04-01T00:00:00Z')

    const noHint = listed(run(updateItemWrite, { itemId: 'i1', read: true }, inbox, items()))
    expect(noHint[0].readAt).toBe('')
  })

  it('applies only the fields a write sets', () => {
    const unread = listed(
      run(updateItemWrite, { itemId: 'i2', read: false, bookmarked: false }, inbox, items(), NOW)
    )
    expect(unread[1]).toMatchObject({ readAt: '', bookmarked: false, dismissed: false })

    const dismissed = listed(
      run(updateItemWrite, { itemId: 'i2', dismissed: true }, inbox, items(), NOW)
    )
    expect(dismissed[0].dismissed).toBe(false)
    expect(dismissed[1]).toMatchObject({
      readAt: '2024-04-01T00:00:00Z',
      bookmarked: true,
      dismissed: true
    })
  })

  it('only raises read progress, capped at 100', () => {
    const pct = (value: number) =>
      listed(run(updateItemWrite, { itemId: 'i1', readProgressPct: value }, inbox, items(), NOW))[0]
        .readProgressPct
    expect(pct(70)).toBe(70)
    expect(pct(10)).toBe(40)
    expect(pct(150)).toBe(100)
  })

  it('patches the summary list and the cached article body', () => {
    expect(
      listed(
        run(updateItemWrite, { itemId: 'i1', bookmarked: true }, swrKeys.feedsSummary, items())
      )[0].bookmarked
    ).toBe(true)

    const body = create(GetFeedItemResponseSchema, { item: { id: 'i1', contentHtml: '<p>x</p>' } })
    expect(
      run(updateItemWrite, { itemId: 'i1', read: true }, swrKeys.feedItem('i1'), body, NOW)
    ).toMatchObject({ item: { contentHtml: '<p>x</p>', readAt: NOW } })
    expect(run(updateItemWrite, { itemId: 'i1', read: true }, swrKeys.feedItem('i2'), body)).toBe(
      body
    )
    const empty = create(GetFeedItemResponseSchema, {})
    expect(run(updateItemWrite, { itemId: 'i1' }, swrKeys.feedItem('i1'), empty)).toBe(empty)
  })

  it('removes a restored item from filtered lists only', () => {
    const filtered = run(
      restoreFeedItemWrite,
      { itemId: 'i1' },
      swrKeys.feedFilteredItems('f1'),
      items()
    )
    expect(listed(filtered).map((i) => i.id)).toEqual(['i2'])

    const data = items()
    expect(run(restoreFeedItemWrite, { itemId: 'i1' }, inbox, data)).toBe(data)
  })

  it('removes a deleted feed with its items, rules and suggestions', () => {
    const feeds = create(ListFeedsResponseSchema, { feeds: [{ id: 'f1' }, { id: 'f2' }] })
    expect(run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feeds, feeds)).toMatchObject({
      feeds: [{ id: 'f2' }]
    })
    expect(listed(run(deleteFeedWrite, { feedId: 'f1' }, inbox, items())).map((i) => i.id)).toEqual(
      ['i2']
    )

    const rules = create(ListFilterRulesResponseSchema, {
      rules: [
        { id: 'r1', feedId: 'f1' },
        { id: 'r2', feedId: '' }
      ]
    })
    expect(run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feedFilterRules, rules)).toMatchObject({
      rules: [{ id: 'r2' }]
    })

    const suggestions = create(GetFilterRuleSuggestionsResponseSchema, {
      suggestions: [
        { feedId: 'f1', category: 'News' },
        { feedId: 'f2', category: 'News' }
      ]
    })
    expect(
      run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feedFilterRuleSuggestions, suggestions)
    ).toMatchObject({ suggestions: [{ feedId: 'f2' }] })

    expect(run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feeds, rules)).toBe(rules)
    expect(run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feedFilterRules, feeds)).toBe(feeds)
    expect(run(deleteFeedWrite, { feedId: 'f1' }, swrKeys.feedFilterRuleSuggestions, feeds)).toBe(
      feeds
    )
  })

  it('removes a deleted rule and a dismissed suggestion', () => {
    const rules = create(ListFilterRulesResponseSchema, { rules: [{ id: 'r1' }, { id: 'r2' }] })
    expect(
      run(deleteFilterRuleWrite, { ruleId: 'r1' }, swrKeys.feedFilterRules, rules)
    ).toMatchObject({ rules: [{ id: 'r2' }] })
    expect(run(deleteFilterRuleWrite, { ruleId: 'r1' }, swrKeys.feeds, rules)).toBe(rules)

    const suggestions = create(GetFilterRuleSuggestionsResponseSchema, {
      suggestions: [
        { feedId: 'f1', category: 'News' },
        { feedId: 'f1', category: 'Sport' },
        { feedId: 'f2', category: 'News' }
      ]
    })
    const result = run(
      dismissSuggestionWrite,
      { feedId: 'f1', category: 'News' },
      swrKeys.feedFilterRuleSuggestions,
      suggestions
    )
    expect(result).toMatchObject({
      suggestions: [
        { feedId: 'f1', category: 'Sport' },
        { feedId: 'f2', category: 'News' }
      ]
    })
    expect(run(dismissSuggestionWrite, { feedId: 'f1' }, swrKeys.feeds, suggestions)).toBe(
      suggestions
    )
  })

  it('describes each write', () => {
    expect(updateItemWrite.describe(updateItemWrite.encode({}))).toBe('Update an article')
    expect(restoreFeedItemWrite.describe(restoreFeedItemWrite.encode({}))).toBe(
      'Restore a filtered article'
    )
    expect(deleteFeedWrite.describe(deleteFeedWrite.encode({}))).toBe('Remove a feed')
    expect(deleteFilterRuleWrite.describe(deleteFilterRuleWrite.encode({}))).toBe(
      'Delete a filter rule'
    )
    expect(
      dismissSuggestionWrite.describe(dismissSuggestionWrite.encode({ category: 'News' }))
    ).toBe('Dismiss the “News” suggestion')
  })

  it('refetches feeds, skipping the refetch after a sent item update', () => {
    expect(feedWrites).toEqual([
      updateItemWrite,
      restoreFeedItemWrite,
      deleteFeedWrite,
      deleteFilterRuleWrite,
      dismissSuggestionWrite
    ])
    for (const write of feedWrites) {
      expect(write.revalidate).toBe('/feeds')
      expect(write.revalidateOnSuccess).toBe(write !== updateItemWrite)
    }
  })
})
