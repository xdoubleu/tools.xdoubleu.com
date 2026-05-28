'use client'

import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'
import { Badge } from '@/components/ui/badge'

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
      <h3 className="font-semibold text-fg">Pending Requests</h3>
      {pending.map((contact) => (
        <div
          key={contact.id}
          className="flex items-center justify-between rounded-2xl border border-border bg-card p-4 shadow-card"
        >
          <div>
            <p className="font-medium text-sm text-fg">
              {contact.displayName || contact.contactUserId}
            </p>
            <p className="text-xs text-muted">{contact.contactUserId}</p>
          </div>
          <Badge variant="warn">Pending</Badge>
        </div>
      ))}
    </div>
  )
}
