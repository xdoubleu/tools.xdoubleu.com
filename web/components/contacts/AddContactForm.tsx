'use client'

import { useState } from 'react'
import { useCreateContact } from '@/hooks/useContacts'

interface AddContactFormProps {
  onSuccess?: () => void
}

export default function AddContactForm({ onSuccess }: AddContactFormProps) {
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const createContact = useCreateContact()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      await createContact(email, displayName || email.split('@')[0])
      setEmail('')
      setDisplayName('')
      onSuccess?.()
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('Failed to create contact')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div>
        <label htmlFor="email" className="block text-sm font-medium text-subtle mb-1">
          Email
        </label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <label htmlFor="displayName" className="block text-sm font-medium text-subtle mb-1">
          Display Name (optional)
        </label>
        <input
          id="displayName"
          type="text"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      <button
        type="submit"
        disabled={submitting}
        className="w-full px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:bg-muted"
      >
        {submitting ? 'Adding...' : 'Add Contact'}
      </button>
    </form>
  )
}
