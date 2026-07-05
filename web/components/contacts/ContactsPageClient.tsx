'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import {
  useContacts,
  useCreateContact,
  useAcceptContact,
  useDeclineContact,
  useUpdateContact,
  useDeleteContact
} from '@/hooks/useContacts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { swrKeys } from '@/lib/swrKeys'
import { PageContainer } from '@/components/ui/page-container'

export default function ContactsPageClient() {
  const { data, isLoading, error } = useContacts()
  const createContact = useCreateContact()
  const acceptContact = useAcceptContact()
  const declineContact = useDeclineContact()
  const updateContact = useUpdateContact()
  const deleteContact = useDeleteContact()

  const [email, setEmail] = useState('')
  const [addError, setAddError] = useState('')
  const [adding, setAdding] = useState(false)
  const [acceptNames, setAcceptNames] = useState<Record<string, string>>({})
  const [editingId, setEditingId] = useState('')
  const [editName, setEditName] = useState('')

  const contacts = data?.contacts ?? []
  const pending = data?.pending ?? []
  const incoming = data?.incoming ?? []

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault()
    setAdding(true)
    setAddError('')
    try {
      await createContact(email, email)
      setEmail('')
      await mutate(swrKeys.contacts)
    } catch {
      setAddError('Failed to add contact. Check the email and try again.')
    } finally {
      setAdding(false)
    }
  }

  async function handleAccept(id: string) {
    const displayName = acceptNames[id] ?? incoming.find((c) => c.id === id)?.ownerEmail ?? ''
    try {
      await acceptContact(id, displayName)
      await mutate(swrKeys.contacts)
    } catch {
      // ignore
    }
  }

  async function handleDecline(id: string) {
    try {
      await declineContact(id)
      await mutate(swrKeys.contacts)
    } catch {
      // ignore
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteContact(id)
      await mutate(swrKeys.contacts)
    } catch {
      // ignore
    }
  }

  async function handleRename(id: string) {
    if (!editName.trim()) return
    try {
      await updateContact(id, editName.trim())
      setEditingId('')
      setEditName('')
      await mutate(swrKeys.contacts)
    } catch {
      // ignore
    }
  }

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-danger">Failed to load contacts.</p>
  }

  return (
    <PageContainer className="max-w-lg p-6">
      <h1 className="mb-6 text-3xl font-bold">Contacts</h1>

      <div className="mb-6 rounded-2xl border border-border bg-card p-4">
        <h2 className="mb-3 text-sm font-semibold text-subtle">Add contact</h2>
        {addError && <p className="mb-2 text-xs text-danger">{addError}</p>}
        <form onSubmit={handleAdd} className="flex gap-2">
          <Input
            type="email"
            required
            placeholder="Email address"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="flex-1"
          />
          <Button type="submit" disabled={adding}>
            {adding ? 'Adding…' : 'Add'}
          </Button>
        </form>
      </div>

      {incoming.length > 0 && (
        <section className="mb-6">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
            Contact requests
          </h2>
          <ul className="space-y-3">
            {incoming.map((c) => (
              <li key={c.id} className="rounded-2xl border border-warn/30 bg-warn/10 p-3">
                <p className="mb-2 text-sm font-semibold text-fg">
                  {c.ownerEmail} wants to connect
                </p>
                <div className="flex items-end gap-2">
                  <div className="flex-1">
                    <label className="mb-1 block text-xs text-muted">Name for them</label>
                    <Input
                      type="text"
                      required
                      defaultValue={c.ownerEmail}
                      onChange={(e) =>
                        setAcceptNames((prev) => ({ ...prev, [c.id]: e.target.value }))
                      }
                    />
                  </div>
                  <Button onClick={() => handleAccept(c.id)}>Accept</Button>
                  <Button variant="secondary" onClick={() => handleDecline(c.id)}>
                    Decline
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </section>
      )}

      {contacts.length > 0 ? (
        <section className="mb-6">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
            Contacts
          </h2>
          <ul className="space-y-2">
            {contacts.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-card px-3 py-2"
              >
                {editingId === c.id ? (
                  <>
                    <Input
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      className="h-9 flex-1"
                      aria-label={`Rename ${c.displayName}`}
                    />
                    <Button size="sm" onClick={() => handleRename(c.id)}>
                      Save
                    </Button>
                    <Button size="sm" variant="secondary" onClick={() => setEditingId('')}>
                      Cancel
                    </Button>
                  </>
                ) : (
                  <>
                    <span className="text-sm font-medium text-fg">{c.displayName}</span>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="link"
                        size="sm"
                        onClick={() => {
                          setEditingId(c.id)
                          setEditName(c.displayName)
                        }}
                        className="h-auto px-0 text-xs"
                      >
                        Rename
                      </Button>
                      <Button
                        variant="link"
                        size="sm"
                        onClick={() => handleDelete(c.id)}
                        className="h-auto px-0 text-xs text-danger focus-visible:ring-danger/50"
                      >
                        Remove
                      </Button>
                    </div>
                  </>
                )}
              </li>
            ))}
          </ul>
        </section>
      ) : (
        incoming.length === 0 && (
          <p className="text-sm text-muted">No contacts yet. Add someone by email above.</p>
        )
      )}

      {pending.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">
            Sent (awaiting acceptance)
          </h2>
          <ul className="space-y-2">
            {pending.map((c) => (
              <li
                key={c.id}
                className="flex items-center justify-between rounded-2xl border border-border bg-card px-3 py-2"
              >
                <span className="text-sm text-muted">{c.displayName}</span>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => handleDelete(c.id)}
                  className="h-auto px-0 text-xs text-subtle hover:text-fg focus-visible:ring-border"
                >
                  Cancel
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </PageContainer>
  )
}
