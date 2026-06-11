'use client'

import { useState } from 'react'
import { useAcceptContact, useDeclineContact } from '@/hooks/useContacts'
import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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
      await acceptContact(id, displayNames[id] || id)
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
      <h3 className="font-semibold text-fg">Incoming Requests</h3>
      {incoming.map((contact) => (
        <div
          key={contact.id}
          className="rounded-2xl border border-border bg-card p-4 shadow-card space-y-2"
        >
          <p className="text-sm text-muted">{contact.ownerUserId}</p>
          <Input
            type="text"
            placeholder="Display name"
            value={displayNames[contact.id] || contact.displayName || ''}
            onChange={(e) => setDisplayNames({ ...displayNames, [contact.id]: e.target.value })}
          />
          <div className="flex gap-2">
            <Button onClick={() => handleAccept(contact.id)} className="flex-1" size="sm">
              Accept
            </Button>
            <Button
              variant="destructive"
              onClick={() => handleDecline(contact.id)}
              className="flex-1"
              size="sm"
            >
              Decline
            </Button>
          </div>
        </div>
      ))}
    </div>
  )
}
