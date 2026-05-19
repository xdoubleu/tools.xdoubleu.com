'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { useSettings } from '@/hooks/useSettings'
import { useSignIn, useMFAEnroll, useMFAEnrollVerify, useMFAChallenge } from '@/hooks/useAuth'
import { ConnectError } from '@connectrpc/connect'

type AuthState = 'loading' | 'authenticated' | 'unauthenticated' | 'mfa-enroll' | 'mfa-challenge'

interface AppLink {
  label: string
  href: string
  description: string
}

const APPS: AppLink[] = [
  { label: 'Backlog', href: '/backlog', description: 'Goals and backlog tracker' },
  { label: 'Watch Party', href: '/watchparty', description: 'WebRTC screen sharing' },
  { label: 'ICS Proxy', href: '/icsproxy', description: 'Calendar feed filtering' },
  { label: 'Recipes', href: '/recipes', description: 'Recipe management' },
  { label: 'Todos', href: '/todos', description: 'Task management' }
]

export default function HomeClient() {
  const { data, error, isLoading } = useSettings()
  const signIn = useSignIn()
  const mFAEnroll = useMFAEnroll()
  const mFAEnrollVerify = useMFAEnrollVerify()
  const mFAChallenge = useMFAChallenge()

  const [authState, setAuthState] = useState<AuthState>('loading')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [signInError, setSignInError] = useState<string | null>(null)
  const [factorId, setFactorId] = useState('')
  const [qrSvg, setQrSvg] = useState('')
  const [mfaSecret, setMfaSecret] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  const [mfaError, setMfaError] = useState<string | null>(null)
  const [mfaSubmitting, setMfaSubmitting] = useState(false)

  useEffect(() => {
    if (!isLoading) {
      if (data) {
        setAuthState('authenticated')
      } else if (error) {
        setAuthState((prev) =>
          prev === 'mfa-enroll' || prev === 'mfa-challenge' ? prev : 'unauthenticated'
        )
      }
    }
  }, [isLoading, data, error])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setSignInError(null)

    try {
      const res = await signIn(email, password, rememberMe, '')
      if (res.needsMfa && res.enrollMfa) {
        try {
          const enrollment = await mFAEnroll()
          setFactorId(enrollment.factorId)
          setQrSvg(enrollment.qrSvg)
          setMfaSecret(enrollment.secret)
          setAuthState('mfa-enroll')
        } catch (enrollErr) {
          if (enrollErr instanceof ConnectError) {
            setSignInError(enrollErr.message)
          } else {
            setSignInError('Failed to initialize MFA enrollment.')
          }
        }
      } else if (res.needsMfa) {
        setAuthState('mfa-challenge')
      } else {
        if (typeof window !== 'undefined') {
          window.location.reload()
        }
      }
    } catch (err) {
      if (err instanceof ConnectError) {
        setSignInError(err.message)
      } else {
        setSignInError('Sign-in failed.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleMfaEnrollVerify = async () => {
    setMfaSubmitting(true)
    setMfaError(null)

    try {
      await mFAEnrollVerify(factorId, mfaCode)
      if (typeof window !== 'undefined') {
        window.location.reload()
      }
    } catch (err) {
      if (err instanceof ConnectError) {
        setMfaError(err.message)
      } else {
        setMfaError('Verification failed.')
      }
    } finally {
      setMfaSubmitting(false)
    }
  }

  const handleMfaChallenge = async () => {
    setMfaSubmitting(true)
    setMfaError(null)

    try {
      await mFAChallenge(mfaCode)
      if (typeof window !== 'undefined') {
        window.location.reload()
      }
    } catch (err) {
      if (err instanceof ConnectError) {
        setMfaError(err.message)
      } else {
        setMfaError('Challenge failed.')
      }
    } finally {
      setMfaSubmitting(false)
    }
  }

  if (authState === 'loading') {
    return <p className="text-muted">Loading...</p>
  }

  if (authState === 'authenticated') {
    return (
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {APPS.map((app) => (
          <Link
            key={app.href}
            href={app.href}
            className={
              'rounded-xl border border-border bg-card p-6 shadow-sm ' +
              'transition-shadow hover:shadow-md'
            }
          >
            <h2 className="text-lg font-semibold text-fg">{app.label}</h2>
            <p className="mt-1 text-sm text-muted">{app.description}</p>
          </Link>
        ))}
      </div>
    )
  }

  if (authState === 'mfa-enroll') {
    return (
      <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-fg">Set up two-factor authentication</h2>
        <div className="mt-6 space-y-4">
          <div
            dangerouslySetInnerHTML={{ __html: qrSvg }}
            aria-label="QR code"
            className="flex justify-center"
          />
          <div>
            <p className="text-sm text-subtle">
              Or enter this code manually:{' '}
              <code className="font-mono text-fg">{mfaSecret}</code>
            </p>
          </div>
          <div>
            <label htmlFor="mfaCode" className="block text-sm font-medium text-subtle">
              Authenticator code
            </label>
            <input
              id="mfaCode"
              type="text"
              inputMode="numeric"
              maxLength={6}
              value={mfaCode}
              onChange={(e) => setMfaCode(e.target.value)}
              className={
                'mt-1 block w-full rounded border border-input-border bg-input px-3 py-2 ' +
                'text-input-text placeholder-muted focus:border-blue-500 ' +
                'focus:outline-none focus:ring-1 focus:ring-blue-500'
              }
            />
          </div>
          {mfaError && (
            <p role="alert" className="text-sm text-red-600">
              {mfaError}
            </p>
          )}
          <button
            onClick={handleMfaEnrollVerify}
            disabled={mfaSubmitting}
            className={
              'w-full rounded bg-blue-600 px-4 py-2 font-medium text-white ' +
              'transition-colors hover:bg-blue-700 disabled:bg-gray-400 ' +
              'disabled:cursor-not-allowed'
            }
          >
            {mfaSubmitting ? 'Verifying...' : 'Verify'}
          </button>
        </div>
      </div>
    )
  }

  if (authState === 'mfa-challenge') {
    return (
      <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-fg">Two-factor authentication</h2>
        <div className="mt-6 space-y-4">
          <p className="text-sm text-subtle">Enter the code from your authenticator app.</p>
          <div>
            <label htmlFor="mfaChallengeCode" className="block text-sm font-medium text-subtle">
              Authenticator code
            </label>
            <input
              id="mfaChallengeCode"
              type="text"
              inputMode="numeric"
              maxLength={6}
              value={mfaCode}
              onChange={(e) => setMfaCode(e.target.value)}
              className={
                'mt-1 block w-full rounded border border-input-border bg-input px-3 py-2 ' +
                'text-input-text placeholder-muted focus:border-blue-500 ' +
                'focus:outline-none focus:ring-1 focus:ring-blue-500'
              }
            />
          </div>
          {mfaError && (
            <p role="alert" className="text-sm text-red-600">
              {mfaError}
            </p>
          )}
          <button
            onClick={handleMfaChallenge}
            disabled={mfaSubmitting}
            className={
              'w-full rounded bg-blue-600 px-4 py-2 font-medium text-white ' +
              'transition-colors hover:bg-blue-700 disabled:bg-gray-400 ' +
              'disabled:cursor-not-allowed'
            }
          >
            {mfaSubmitting ? 'Verifying...' : 'Verify'}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
      <h2 className="text-lg font-semibold text-fg">Sign In</h2>
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
            className={
              'mt-1 block w-full rounded border border-input-border bg-input px-3 py-2 ' +
              'text-input-text placeholder-muted focus:border-blue-500 ' +
              'focus:outline-none focus:ring-1 focus:ring-blue-500'
            }
          />
        </div>

        <div>
          <label htmlFor="password" className="block text-sm font-medium text-subtle">
            Password
          </label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            className={
              'mt-1 block w-full rounded border border-input-border bg-input px-3 py-2 ' +
              'text-input-text placeholder-muted focus:border-blue-500 ' +
              'focus:outline-none focus:ring-1 focus:ring-blue-500'
            }
          />
        </div>

        <div className="flex items-center">
          <input
            id="rememberMe"
            type="checkbox"
            checked={rememberMe}
            onChange={(e) => setRememberMe(e.target.checked)}
            className={'h-4 w-4 rounded border-input-border text-blue-600 ' + 'focus:ring-blue-500'}
          />
          <label htmlFor="rememberMe" className="ml-2 text-sm text-subtle">
            Remember me
          </label>
        </div>

        {signInError && (
          <p role="alert" className="text-sm text-red-600">
            {signInError}
          </p>
        )}

        <button
          type="submit"
          disabled={submitting}
          className={
            'w-full rounded bg-blue-600 px-4 py-2 font-medium text-white ' +
            'transition-colors hover:bg-blue-700 disabled:bg-gray-400 ' +
            'disabled:cursor-not-allowed'
          }
        >
          {submitting ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
