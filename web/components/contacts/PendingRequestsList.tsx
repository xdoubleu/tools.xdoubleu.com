'use client'

import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'

interface PendingRequestsListProps {
  contacts: Contact[]
}

export default function PendingRequestsList({ contacts }: PendingRequestsListProps) {
  const pending = contacts.filter((c) => c.status === 'pending')

  if (pending.length === 0) {
    return <p className="text-muted">No pending requests.</p>
  }

  return (
    <div className="space-y-3">
      <h3 className="font-semibold">Pending Requests</h3>
      {pending.map((contact) => (
        <div key={contact.id} className="border border-border rounded p-4 flex items-center justify-between">
          <div>
            <p className="font-medium text-sm">{contact.displayName || contact.contactUserId}</p>
            <p className="text-xs text-muted">{contact.contactUserId}</p>
          </div>
          <span className="px-3 py-1 bg-yellow-100 text-yellow-800 text-xs rounded">
            Pending
          </span>
        </div>
      ))}
    </div>
  )
}
