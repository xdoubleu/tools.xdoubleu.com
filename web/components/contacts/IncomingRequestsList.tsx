'use client'

import { useState } from 'react'
import { useAcceptContact, useDeclineContact } from '@/hooks/useContacts'
import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'

interface IncomingRequestsListProps {
  contacts: Contact[]
  onUpdated?: () => void
}

export default function IncomingRequestsList({ contacts, onUpdated }: IncomingRequestsListProps) {
  const [displayNames, setDisplayNames] = useState<{ [key: string]: string }>({})

  const acceptContact = useAcceptContact()
  const declineContact = useDeclineContact()

  const incoming = contacts.filter((c) => c.status === 'incoming')

  if (incoming.length === 0) {
    return <p className="text-muted">No incoming requests.</p>
  }

  const handleAccept = async (id: string) => {
    try {
      const displayName = displayNames[id] || id
      await acceptContact(id, displayName)
      onUpdated?.()
    } catch (err) {
      console.error('Failed to accept contact:', err)
    }
  }

  const handleDecline = async (id: string) => {
    try {
      await declineContact(id)
      onUpdated?.()
    } catch (err) {
      console.error('Failed to decline contact:', err)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="font-semibold">Incoming Requests</h3>
      {incoming.map((contact) => (
        <div key={contact.id} className="border border-border rounded p-4 space-y-2">
          <p className="text-sm text-muted">{contact.contactUserId}</p>
          <input
            type="text"
            placeholder="Display name"
            value={displayNames[contact.id] || contact.displayName || ''}
            onChange={(e) => setDisplayNames({ ...displayNames, [contact.id]: e.target.value })}
            className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text text-sm"
          />
          <div className="flex gap-2">
            <button
              onClick={() => handleAccept(contact.id)}
              className="flex-1 px-3 py-1 bg-green-600 text-white text-sm rounded hover:bg-green-700"
            >
              Accept
            </button>
            <button
              onClick={() => handleDecline(contact.id)}
              className="flex-1 px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
            >
              Decline
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}
