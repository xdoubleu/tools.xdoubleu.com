'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { ConnectError } from '@connectrpc/connect'
import { useResetPassword } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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
    <div className="flex justify-center py-8">
      <div className="w-full max-w-sm">
        <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-fg">Set new password</h2>

          {state === 'invalid' && (
            <div className="mt-6 space-y-4">
              <p className="text-sm text-danger">{error ?? 'Invalid or expired reset link.'}</p>
              <Link
                href="/auth/forgot-password"
                className="block text-center text-sm text-accent hover:underline"
              >
                Request a new reset link
              </Link>
            </div>
          )}

          {state === 'done' && (
            <div className="mt-6 space-y-4">
              <p className="text-sm text-subtle">Your password has been updated successfully.</p>
              <Link href="/" className="block text-center text-sm text-accent hover:underline">
                Continue to app
              </Link>
            </div>
          )}

          {state === 'form' && (
            <form onSubmit={handleSubmit} className="mt-6 space-y-4">
              <div>
                <label htmlFor="new_password" className="block text-sm font-medium text-subtle">
                  New password
                </label>
                <Input
                  id="new_password"
                  type="password"
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  className="mt-1"
                />
              </div>
              <div>
                <label htmlFor="confirm_password" className="block text-sm font-medium text-subtle">
                  Confirm new password
                </label>
                <Input
                  id="confirm_password"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  className="mt-1"
                />
              </div>

              {error && (
                <div
                  role="alert"
                  className="rounded-xl border border-danger/30 bg-danger/10 px-4 py-2 text-sm text-danger"
                >
                  {error}
                </div>
              )}

              <Button type="submit" disabled={submitting} className="w-full">
                {submitting ? 'Updating…' : 'Update password'}
              </Button>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
