'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { ConnectError } from '@connectrpc/connect'
import { useResetPassword } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

type State = 'form' | 'done' | 'invalid'

export default function ResetPasswordPage() {
  const resetPassword = useResetPassword()
  const searchParams = useSearchParams()
  const token = searchParams.get('token')

  const [state, setState] = useState<State>(token ? 'form' : 'invalid')
  const [error, setError] = useState<string | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    if (!token) {
      setState('invalid')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.')
      return
    }
    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters.')
      return
    }
    setSubmitting(true)
    try {
      await resetPassword(token, newPassword)
      setState('done')
    } catch (err) {
      if (err instanceof ConnectError) {
        setError(err.message)
      } else {
        setError('Failed to update password. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <PageContainer size="narrow">
      <PageHeader title="Set new password" />
      <Card className="p-6">
        {state === 'invalid' && (
          <div className="space-y-4">
            <Alert tone="danger">{error ?? 'Invalid or expired reset link.'}</Alert>
            <Button asChild variant="link" className="w-full">
              <Link href="/auth/forgot-password">Request a new reset link</Link>
            </Button>
          </div>
        )}

        {state === 'done' && (
          <div className="space-y-4">
            <Alert tone="success">Your password has been updated successfully.</Alert>
            <Button asChild variant="link" className="w-full">
              <Link href="/">Continue to app</Link>
            </Button>
          </div>
        )}

        {state === 'form' && (
          <form onSubmit={handleSubmit} className="space-y-4">
            <Field label="New password" htmlFor="new_password">
              <Input
                id="new_password"
                type="password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
              />
            </Field>
            <Field label="Confirm new password" htmlFor="confirm_password">
              <Input
                id="confirm_password"
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
              />
            </Field>

            {error && <Alert tone="danger">{error}</Alert>}

            <Button type="submit" disabled={submitting} className="w-full">
              {submitting ? 'Updating…' : 'Update password'}
            </Button>
          </form>
        )}
      </Card>
    </PageContainer>
  )
}
