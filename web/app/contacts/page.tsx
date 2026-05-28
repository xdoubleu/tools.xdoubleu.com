'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import {
  useContacts,
  useCreateContact,
  useAcceptContact,
  useDeclineContact,
  useDeleteContact
} from '@/hooks/useContacts'

export default function ContactsPage() {
  const { data, isLoading, error } = useContacts()
  const createContact = useCreateContact()
  const acceptContact = useAcceptContact()
  const declineContact = useDeclineContact()
  const deleteContact = useDeleteContact()

  const [email, setEmail] = useState('')
  const [addError, setAddError] = useState('')
  const [adding, setAdding] = useState(false)
  const [acceptNames, setAcceptNames] = useState<Record<string, string>>({})

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
      await mutate('/contacts')
    } catch {
      setAddError('Failed to add contact. Check the email and try again.')
    } finally {
      setAdding(false)
    }
  }

  async function handleAccept(id: string) {
    const displayName = acceptNames[id] ?? ''
    try {
      await acceptContact(id, displayName)
      await mutate('/contacts')
    } catch {
      // ignore
    }
  }

  async function handleDecline(id: string) {
    try {
      await declineContact(id)
      await mutate('/contacts')
    } catch {
      // ignore
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteContact(id)
      await mutate('/contacts')
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
    <main className="mx-auto max-w-lg px-4 py-10">
      <h1 className="mb-6 text-xl font-semibold text-fg">Contacts</h1>

      <div className="mb-6 rounded border border-border bg-card p-4">
        <h2 className="mb-3 text-sm font-semibold text-subtle">Add contact</h2>
        {addError && <p className="mb-2 text-xs text-danger">{addError}</p>}
        <form onSubmit={handleAdd} className="flex gap-2">
          <input
            type="email"
            required
            placeholder="Email address"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="flex-1 rounded border border-border bg-surface px-3 py-2 text-sm text-fg placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-fg"
          />
          <button
            type="submit"
            disabled={adding}
            className="rounded bg-fg px-4 py-2 text-sm font-medium text-bg hover:opacity-80 disabled:opacity-50"
          >
            {adding ? 'Adding…' : 'Add'}
          </button>
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
                  {c.displayName} wants to connect
                </p>
                <div className="flex items-end gap-2">
                  <div className="flex-1">
                    <label className="mb-1 block text-xs text-muted">Name for them</label>
                    <input
                      type="text"
                      required
                      defaultValue={c.displayName}
                      onChange={(e) =>
                        setAcceptNames((prev) => ({ ...prev, [c.id]: e.target.value }))
                      }
                      className="h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
                    />
                  </div>
                  <button
                    onClick={() => handleAccept(c.id)}
                    className="rounded bg-fg px-3 py-1.5 text-sm font-medium text-bg hover:opacity-80"
                  >
                    Accept
                  </button>
                  <button
                    onClick={() => handleDecline(c.id)}
                    className="rounded border border-border bg-surface px-3 py-1.5 text-sm text-subtle hover:bg-bg"
                  >
                    Decline
                  </button>
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
                className="flex items-center justify-between rounded border border-border bg-card px-3 py-2"
              >
                <span className="text-sm font-medium text-fg">{c.displayName}</span>
                <button
                  onClick={() => handleDelete(c.id)}
                  className="text-xs text-danger hover:underline"
                >
                  Remove
                </button>
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
                className="flex items-center justify-between rounded border border-border bg-card px-3 py-2"
              >
                <span className="text-sm text-muted">{c.displayName}</span>
                <button
                  onClick={() => handleDelete(c.id)}
                  className="text-xs text-subtle hover:underline"
                >
                  Cancel
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </main>
  )
}
