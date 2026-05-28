'use client'

import { useDeleteContact } from '@/hooks/useContacts'
import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'
import { Button } from '@/components/ui/button'

interface ContactsListProps {
  contacts: Contact[]
  onUpdated?: () => void
}

export default function ContactsList({ contacts, onUpdated }: ContactsListProps) {
  const deleteContact = useDeleteContact()

  const confirmed = contacts.filter((c) => c.status === 'confirmed')

  if (confirmed.length === 0) {
    return <p className="text-muted">No contacts yet.</p>
  }

  const handleDelete = async (id: string) => {
    if (window.confirm('Are you sure?')) {
      try {
        await deleteContact(id)
        onUpdated?.()
      } catch (err) {
        console.error('Failed to delete contact:', err)
      }
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="font-semibold text-fg">Contacts</h3>
      {confirmed.map((contact) => (
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
          <Button variant="destructive" size="sm" onClick={() => handleDelete(contact.id)}>
            Remove
          </Button>
        </div>
      ))}
    </div>
  )
}
