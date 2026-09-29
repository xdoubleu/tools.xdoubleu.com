'use client'

import { useId, useState } from 'react'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useCreateFilterRule } from '@/hooks/useFeeds'
import {
  useDismissFilterRuleSuggestion,
  useFilterRuleSuggestions
} from '@/hooks/useFeedRuleSuggestions'
import { feedLabel } from '@/lib/feeds/feedLabel'
import { createRuleErrorMessage, filteredMessage } from '@/lib/feeds/filterRuleMessages'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type { FilterRuleSuggestion } from '@/lib/gen/feeds/v1/feeds_pb'

interface Status {
  ok: boolean
  message: string
}

type ReportStatus = (status: Status | null) => void

function SuggestionRow({
  suggestion,
  onStatus
}: {
  suggestion: FilterRuleSuggestion
  onStatus: ReportStatus
}) {
  const createRule = useCreateFilterRule()
  const dismiss = useDismissFilterRuleSuggestion()
  const [pending, setPending] = useState<'create' | 'dismiss' | null>(null)
  const textId = useId()

  const create = async () => {
    setPending('create')
    onStatus(null)
    try {
      const resp = await createRule({
        feedId: suggestion.feedId,
        kind: FilterRuleKind.CATEGORY,
        value: suggestion.category
      })
      onStatus({ ok: true, message: filteredMessage(resp.rule?.filteredCount ?? 0) })
    } catch (err) {
      onStatus({ ok: false, message: createRuleErrorMessage(err) })
    } finally {
      setPending(null)
    }
  }

  const skip = async () => {
    setPending('dismiss')
    onStatus(null)
    try {
      await dismiss(suggestion.feedId, suggestion.category)
    } catch {
      onStatus({ ok: false, message: 'Dismissing the suggestion failed. Please try again.' })
    } finally {
      setPending(null)
    }
  }

  return (
    <li>
      <Card variant="inset" className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <p id={textId} className="min-w-0 flex-1 break-words text-sm">
          {feedLabel({ title: suggestion.feedTitle, url: suggestion.feedUrl })} ·{' '}
          <em>{suggestion.category}</em> · read {suggestion.readCount} of {suggestion.itemCount}
        </p>
        <div className="flex gap-2">
          <Button
            size="sm"
            disabled={pending !== null}
            aria-describedby={textId}
            onClick={() => void create()}
          >
            {pending === 'create' ? 'Adding…' : 'Create rule'}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={pending !== null}
            aria-describedby={textId}
            onClick={() => void skip()}
          >
            {pending === 'dismiss' ? 'Dismissing…' : 'Dismiss'}
          </Button>
        </div>
      </Card>
    </li>
  )
}

// Suggests category rules for feed categories the user rarely reads; hidden
// while there are none.
export default function FeedRuleSuggestionsCard({ className }: { className?: string }) {
  const { data } = useFilterRuleSuggestions()
  const [status, setStatus] = useState<Status | null>(null)
  const suggestions = data?.suggestions ?? []

  if (suggestions.length === 0 && !status) return null

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>Suggested filters</CardTitle>
        <CardDescription>
          Categories you rarely read in the last 90 days. A rule hides their unread items from the
          inbox; nothing is filtered until you create one.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {status && <Alert tone={status.ok ? 'success' : 'danger'}>{status.message}</Alert>}
        {suggestions.length > 0 && (
          <ul className="space-y-2">
            {suggestions.map((s) => (
              <SuggestionRow
                key={`${s.feedId}:${s.category}`}
                suggestion={s}
                onStatus={setStatus}
              />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
