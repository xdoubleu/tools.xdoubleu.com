'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useForgotPassword } from '@/hooks/useAuth'
import { ConnectError } from '@connectrpc/connect'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

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
    <PageContainer size="narrow">
      <PageHeader title="Reset your password" />
      <Card className="p-6">
        {sent ? (
          <div className="space-y-4">
            <p className="text-sm text-subtle">
              If an account with that email exists, you will receive a password reset link shortly.
            </p>
            <Button asChild variant="link" className="w-full">
              <Link href="/auth/sign-in">Back to sign in</Link>
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <Field label="Email" htmlFor="email">
              <Input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </Field>

            {error && (
              <p role="alert" className="text-sm text-danger">
                {error}
              </p>
            )}

            <Button type="submit" disabled={submitting} className="w-full">
              {submitting ? 'Sending…' : 'Send reset link'}
            </Button>

            <div className="text-center">
              <Button asChild variant="link">
                <Link href="/auth/sign-in">Back to sign in</Link>
              </Button>
            </div>
          </form>
        )}
      </Card>
    </PageContainer>
  )
}
