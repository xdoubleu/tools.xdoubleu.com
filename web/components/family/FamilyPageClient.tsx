'use client'

import { useEffect, useState } from 'react'
import { mutate } from 'swr'
import {
  useFamily,
  useInviteToFamily,
  useAcceptFamilyInvite,
  useDeclineFamilyInvite,
  useSetFamilyDisplayName,
  useLeaveFamily
} from '@/hooks/useFamily'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { SectionCard } from '@/components/ui/section-card'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function FamilyPageClient() {
  const { data, isLoading, error } = useFamily()
  const inviteToFamily = useInviteToFamily()
  const acceptInvite = useAcceptFamilyInvite()
  const declineInvite = useDeclineFamilyInvite()
  const setDisplayName = useSetFamilyDisplayName()
  const leaveFamily = useLeaveFamily()

  const [email, setEmail] = useState('')
  const [inviteError, setInviteError] = useState('')
  const [inviting, setInviting] = useState(false)
  const [accepting, setAccepting] = useState(false)
  const [declining, setDeclining] = useState(false)
  const [leaving, setLeaving] = useState(false)

  const [name, setName] = useState('')
  const [savingName, setSavingName] = useState(false)

  const members = data?.members ?? []
  const incomingInvite = data?.incomingInvite
  const selfDisplayName = data?.selfDisplayName ?? ''

  useEffect(() => {
    setName(selfDisplayName)
  }, [selfDisplayName])

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

  async function handleSaveName(e: React.FormEvent) {
    e.preventDefault()
    setSavingName(true)
    try {
      await setDisplayName(name.trim())
      await mutate(swrKeys.family)
    } catch {
      // ignore
    } finally {
      setSavingName(false)
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
    return <LoadingState className="py-16 text-center text-sm" />
  }

  if (error) {
    return <ErrorState what="family" className="py-16 text-center text-sm" />
  }

  return (
    <PageContainer size="narrow" className="space-y-6">
      <PageHeader
        title="Family"
        description="A family shares one recipe book, one meal plan and one shopping list together — not separate lists cross-shared, but a single set everyone in it sees and edits."
        className="mb-0"
      />

      {incomingInvite && (
        <Alert tone="warn" className="space-y-3 p-4">
          <p className="break-all font-semibold">
            {incomingInvite.fromEmail} invited you to join their family
          </p>
          <div className="flex flex-wrap gap-2">
            <Button onClick={handleAccept} disabled={accepting}>
              {accepting ? 'Accepting…' : 'Accept'}
            </Button>
            <Button variant="secondary" onClick={handleDecline} disabled={declining}>
              {declining ? 'Declining…' : 'Decline'}
            </Button>
          </div>
        </Alert>
      )}

      <SectionCard title="Invite to your family">
        {inviteError && <p className="mb-2 text-xs text-danger">{inviteError}</p>}
        <form onSubmit={handleInvite} className="flex gap-2">
          <Input
            type="email"
            required
            autoComplete="off"
            aria-label="Email address"
            placeholder="Email address"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="min-w-0 flex-1"
          />
          <Button type="submit" disabled={inviting}>
            {inviting ? 'Inviting…' : 'Invite'}
          </Button>
        </form>
      </SectionCard>

      <SectionCard
        title="Your name"
        description="Shown to the rest of your family in place of your email."
      >
        <form onSubmit={handleSaveName} className="flex gap-2">
          <Input
            type="text"
            aria-label="Your name"
            placeholder="Your name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="min-w-0 flex-1"
          />
          <Button type="submit" disabled={savingName || name.trim() === selfDisplayName}>
            {savingName ? 'Saving…' : 'Save'}
          </Button>
        </form>
      </SectionCard>

      <SectionCard title="Members">
        {members.length > 0 ? (
          <ul className="mb-4 space-y-2">
            {members.map((m) => (
              <li key={m.userId}>
                <Card variant="inset" className="break-all text-sm font-medium text-fg">
                  {m.displayName || m.email}
                </Card>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mb-4 text-sm text-muted">
            Just you for now. Invite someone by email to share your recipes, meal plans and shopping
            list with them.
          </p>
        )}

        {members.length > 0 && (
          <Button
            variant="link"
            size="sm"
            onClick={handleLeave}
            disabled={leaving}
            className="text-xs text-danger focus-visible:ring-danger/50"
          >
            {leaving ? 'Leaving…' : 'Leave family'}
          </Button>
        )}
      </SectionCard>
    </PageContainer>
  )
}
