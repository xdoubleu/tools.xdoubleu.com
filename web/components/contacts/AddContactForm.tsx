'use client'

import { useState } from 'react'
import { useCreateContact } from '@/hooks/useContacts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

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
      setError(err instanceof Error ? err.message : 'Failed to create contact')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      <div className="space-y-1.5">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="displayName">Display Name (optional)</Label>
        <Input
          id="displayName"
          type="text"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
      </div>

      {error && <p className="text-sm text-danger">{error}</p>}

      <Button type="submit" disabled={submitting} className="w-full">
        {submitting ? 'Adding…' : 'Add Contact'}
      </Button>
    </form>
  )
}
