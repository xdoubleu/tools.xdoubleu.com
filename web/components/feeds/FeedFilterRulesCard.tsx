'use client'

import Link from 'next/link'
import { useState } from 'react'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { ErrorState, LoadingState } from '@/components/ui/states'
import {
  useCreateFilterRule,
  useDeleteFilterRule,
  useFeeds,
  useFilterRules
} from '@/hooks/useFeeds'
import { feedLabel } from '@/lib/feeds/feedLabel'
import { createRuleErrorMessage, filteredMessage } from '@/lib/feeds/filterRuleMessages'
import { FilterRuleKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type { FilterRule } from '@/lib/gen/feeds/v1/feeds_pb'

function kindLabel(kind: FilterRuleKind): string {
  return kind === FilterRuleKind.CATEGORY ? 'Category' : 'Title contains'
}

function RuleRow({ rule, scope }: { rule: FilterRule; scope: string }) {
  const deleteRule = useDeleteFilterRule()
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const kind = kindLabel(rule.kind)

  const remove = async () => {
    setBusy(true)
    setFailed(false)
    try {
      await deleteRule(rule.id)
    } catch {
      setFailed(true)
      setBusy(false)
    }
  }

  return (
    <li>
      <Card variant="inset" className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <p className="break-words text-sm font-medium">{rule.value}</p>
          <div className="flex flex-wrap items-center gap-x-1 text-xs text-muted">
            <span className="break-words">
              {kind} · {scope} ·{' '}
            </span>
            <Button asChild variant="link" className="text-xs">
              <Link href={rule.feedId ? `/feeds/filtered?feed=${rule.feedId}` : '/feeds/filtered'}>
                {rule.filteredCount} filtered
              </Link>
            </Button>
          </div>
          {failed && <p className="text-xs text-danger">Deleting the rule failed.</p>}
        </div>
        <Button
          size="sm"
          variant="destructive"
          disabled={busy}
          aria-label={`Delete rule ${kind}: ${rule.value}`}
          onClick={() => void remove()}
        >
          Delete
        </Button>
      </Card>
    </li>
  )
}

// Rules hiding feed items by category or title, per feed or across all feeds.
export default function FeedFilterRulesCard() {
  const { data: feedsData } = useFeeds()
  const { data, error, isLoading } = useFilterRules()
  const createRule = useCreateFilterRule()
  const [feedId, setFeedId] = useState('')
  const [kind, setKind] = useState<FilterRuleKind>(FilterRuleKind.TITLE)
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState<{ ok: boolean; message: string } | null>(null)

  const feeds = feedsData?.feeds ?? []
  const rules = data?.rules ?? []
  const scopeOf = (rule: FilterRule) => {
    if (!rule.feedId) return 'All feeds'
    const feed = feeds.find((f) => f.id === rule.feedId)
    return feed ? feedLabel(feed) : 'Unknown feed'
  }

  const submit = async () => {
    if (!value.trim() || busy) return
    setBusy(true)
    setStatus(null)
    try {
      const resp = await createRule({ feedId, kind, value: value.trim() })
      setStatus({ ok: true, message: filteredMessage(resp.rule?.filteredCount ?? 0) })
      setValue('')
    } catch (err) {
      setStatus({ ok: false, message: createRuleErrorMessage(err) })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Filter rules</CardTitle>
        <CardDescription>
          Hide items from the inbox, unread counts and stats. A new rule also hides matching unread
          items you already have; read, bookmarked and dismissed items are never touched.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          className="grid grid-cols-1 gap-3 sm:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault()
            void submit()
          }}
        >
          <Field label="Applies to" htmlFor="filter-rule-scope">
            <Select
              id="filter-rule-scope"
              value={feedId}
              onChange={(e) => setFeedId(e.target.value)}
            >
              <option value="">All feeds</option>
              {feeds.map((feed) => (
                <option key={feed.id} value={feed.id}>
                  {feedLabel(feed)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Match" htmlFor="filter-rule-kind">
            <Select
              id="filter-rule-kind"
              value={String(kind)}
              onChange={(e) => setKind(Number(e.target.value) as FilterRuleKind)}
            >
              <option value={String(FilterRuleKind.TITLE)}>Title contains</option>
              <option value={String(FilterRuleKind.CATEGORY)}>Category</option>
            </Select>
          </Field>
          <Field
            label="Value"
            htmlFor="filter-rule-value"
            hint="Matched ignoring case; a category must match exactly."
            className="sm:col-span-2"
          >
            <Input
              id="filter-rule-value"
              value={value}
              maxLength={200}
              onChange={(e) => setValue(e.target.value)}
            />
          </Field>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={busy || !value.trim()}>
              {busy ? 'Adding…' : 'Add rule'}
            </Button>
          </div>
        </form>
        {status && <Alert tone={status.ok ? 'success' : 'danger'}>{status.message}</Alert>}

        {isLoading && <LoadingState label="filter rules" />}
        {error && <ErrorState what="filter rules" />}
        {!isLoading && !error && rules.length === 0 && (
          <p className="text-sm text-muted">No filter rules yet.</p>
        )}
        {rules.length > 0 && (
          <ul className="space-y-2">
            {rules.map((rule) => (
              <RuleRow key={rule.id} rule={rule} scope={scopeOf(rule)} />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
