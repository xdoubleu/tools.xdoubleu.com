'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import {
  useFamily,
  useInviteToFamily,
  useAcceptFamilyInvite,
  useDeclineFamilyInvite,
  useLeaveFamily
} from '@/hooks/useFamily'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'

export default function FamilyPageClient() {
  const { data, isLoading, error } = useFamily()
  const inviteToFamily = useInviteToFamily()
  const acceptInvite = useAcceptFamilyInvite()
  const declineInvite = useDeclineFamilyInvite()
  const leaveFamily = useLeaveFamily()

  const [email, setEmail] = useState('')
  const [inviteError, setInviteError] = useState('')
  const [inviting, setInviting] = useState(false)
  const [accepting, setAccepting] = useState(false)
  const [declining, setDeclining] = useState(false)
  const [leaving, setLeaving] = useState(false)

  const members = data?.members ?? []
  const incomingInvite = data?.incomingInvite

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault()
    setInviting(true)
    setInviteError('')
    try {
      await inviteToFamily(email)
      setEmail('')
      await mutate(swrKeys.family)
    } catch {
      setInviteError('Failed to send invite. Check the email and try again.')
    } finally {
      setInviting(false)
    }
  }

  async function handleAccept() {
    setAccepting(true)
    try {
      await acceptInvite()
      await mutate(swrKeys.family)
    } catch {
      // ignore
    } finally {
      setAccepting(false)
    }
  }

  async function handleDecline() {
    setDeclining(true)
    try {
      await declineInvite()
      await mutate(swrKeys.family)
    } catch {
      // ignore
    } finally {
      setDeclining(false)
    }
  }

  async function handleLeave() {
    setLeaving(true)
    try {
      await leaveFamily()
      await mutate(swrKeys.family)
    } catch {
      // ignore
    } finally {
      setLeaving(false)
    }
  }

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-danger">Failed to load family.</p>
  }

  return (
    <PageContainer className="max-w-lg p-6">
      <h1 className="mb-2 text-3xl font-bold">Family</h1>
      <p className="mb-6 text-sm text-muted">
        A family shares one recipe book, one meal plan and one shopping list together — not separate
        lists cross-shared, but a single set everyone in it sees and edits.
      </p>

      {incomingInvite && (
        <section className="mb-6 rounded-2xl border border-warn/30 bg-warn/10 p-4">
          <p className="mb-3 text-sm font-semibold text-fg">
            {incomingInvite.fromEmail} invited you to join their family
          </p>
          <div className="flex gap-2">
            <Button onClick={handleAccept} disabled={accepting}>
              {accepting ? 'Accepting…' : 'Accept'}
            </Button>
            <Button variant="secondary" onClick={handleDecline} disabled={declining}>
              {declining ? 'Declining…' : 'Decline'}
            </Button>
          </div>
        </section>
      )}

      <div className="mb-6 rounded-2xl border border-border bg-card p-4">
        <h2 className="mb-3 text-sm font-semibold text-subtle">Invite to your family</h2>
        {inviteError && <p className="mb-2 text-xs text-danger">{inviteError}</p>}
        <form onSubmit={handleInvite} className="flex gap-2">
          <Input
            type="email"
            required
            placeholder="Email address"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="flex-1"
          />
          <Button type="submit" disabled={inviting}>
            {inviting ? 'Inviting…' : 'Invite'}
          </Button>
        </form>
      </div>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Members</h2>
        {members.length > 0 ? (
          <ul className="mb-4 space-y-2">
            {members.map((m) => (
              <li
                key={m.userId}
                className="rounded-2xl border border-border bg-card px-3 py-2 text-sm font-medium text-fg"
              >
                {m.email}
              </li>
            ))}
          </ul>
        ) : (
          <p className="mb-4 text-sm text-muted">
            Just you for now. Invite someone from your contacts to share your recipes, meal plans
            and shopping list with them.
          </p>
        )}

        {members.length > 0 && (
          <Button
            variant="link"
            size="sm"
            onClick={handleLeave}
            disabled={leaving}
            className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
          >
            {leaving ? 'Leaving…' : 'Leave family'}
          </Button>
        )}
      </section>
    </PageContainer>
  )
}
