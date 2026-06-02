'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useForgotPassword } from '@/hooks/useAuth'
import { ConnectError } from '@connectrpc/connect'

export default function ForgotPasswordPage() {
  const forgotPassword = useForgotPassword()

  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      await forgotPassword(email)
      setSent(true)
    } catch (err) {
      if (err instanceof ConnectError) {
        setError(err.message)
      } else {
        setError('Something went wrong. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
      <h2 className="text-lg font-semibold text-fg">Reset your password</h2>

      {sent ? (
        <div className="mt-6 space-y-4">
          <p className="text-sm text-subtle">
            If an account with that email exists, you will receive a password reset link shortly.
          </p>
          <Link
            href="/auth/sign-in"
            className="block text-center text-sm text-accent hover:underline"
          >
            Back to sign in
          </Link>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-subtle">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="mt-1 h-11 block w-full rounded-xl border border-input-border bg-input px-3 py-2 text-input-text placeholder:text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            />
          </div>

          {error && (
            <p role="alert" className="text-sm text-danger">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="h-11 w-full rounded-xl bg-accent px-4 font-medium text-white transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submitting ? 'Sending…' : 'Send reset link'}
          </button>

          <div className="text-center">
            <Link href="/auth/sign-in" className="text-sm text-accent hover:underline">
              Back to sign in
            </Link>
          </div>
        </form>
      )}
    </div>
  )
}
