'use client'

import { useDeleteContact } from '@/hooks/useContacts'
import type { Contact } from '@/lib/gen/contacts/v1/contacts_pb'

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
      <h3 className="font-semibold">Contacts</h3>
      {confirmed.map((contact) => (
        <div
          key={contact.id}
          className="border border-border rounded p-4 flex items-center justify-between"
        >
          <div>
            <p className="font-medium text-sm">{contact.displayName || contact.contactUserId}</p>
            <p className="text-xs text-muted">{contact.contactUserId}</p>
          </div>
          <button
            onClick={() => handleDelete(contact.id)}
            className="px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
          >
            Remove
          </button>
        </div>
      ))}
    </div>
  )
}
