'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { useCurrentUser, useSignIn, useMFAChallenge } from '@/hooks/useAuth'
import AppGrid, { type AppLink, type AppSection } from '@/components/AppGrid'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ConnectError } from '@connectrpc/connect'

type AuthState = 'loading' | 'authenticated' | 'unauthenticated' | 'mfa-challenge'

const ALL_APPS: AppLink[] = [
  {
    name: 'games',
    label: 'Games',
    href: '/games',
    description: 'Steam backlog, progress and distribution.'
  },
  {
    name: 'books',
    label: 'Books',
    href: '/books',
    description: 'Search, library and reading progress.'
  },
  { name: 'todos', label: 'Todos', href: '/todos', description: 'Task management' },
  { name: 'recipes', label: 'Recipes', href: '/recipes/list', description: 'Recipe management' },
  {
    name: 'mealplans',
    label: 'Meal Plans',
    href: '/mealplans',
    description: 'Weekly meal planning'
  },
  {
    name: 'shoppinglist',
    label: 'Shopping List',
    href: '/shoppinglist',
    description: 'Generate shopping lists from meal plans'
  },
  {
    name: 'watchparty',
    label: 'Watch Party',
    href: '/watchparty',
    description: 'WebRTC screen sharing'
  },
  {
    name: 'icsproxy',
    label: 'ICS Proxy',
    href: '/icsproxy',
    description: 'Calendar feed filtering'
  },
  { name: 'settings', label: 'Settings', href: '/settings', description: 'User preferences' },
  { name: 'contacts', label: 'Contacts', href: '/contacts', description: 'Manage contacts' },
  {
    name: 'sharing',
    label: 'Sharing',
    href: '/sharing',
    description: 'Manage what you share'
  },
  { name: 'admin', label: 'Admin', href: '/admin', description: 'Administration' }
]

const APP_MAP = new Map(ALL_APPS.map((a) => [a.name, a]))

const SECTION_DEFS: { title: string; names: string[] }[] = [
  { title: 'Productivity', names: ['games', 'books', 'todos'] },
  { title: 'Food', names: ['recipes', 'mealplans', 'shoppinglist'] },
  { title: 'Tools', names: ['watchparty', 'icsproxy'] },
  { title: 'Account', names: ['settings', 'contacts', 'sharing'] },
  { title: 'Admin', names: ['admin'] }
]

const ALWAYS_VISIBLE = new Set(['settings', 'contacts', 'sharing'])
const ADMIN_ONLY = new Set(['admin'])

export default function HomeClient() {
  const { data, error, isLoading } = useCurrentUser()
  const signIn = useSignIn()
  const mFAChallenge = useMFAChallenge()

  const [authState, setAuthState] = useState<AuthState>('loading')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [signInError, setSignInError] = useState<string | null>(null)
  const [mfaCode, setMfaCode] = useState('')
  const [mfaError, setMfaError] = useState<string | null>(null)
  const [mfaSubmitting, setMfaSubmitting] = useState(false)

  useEffect(() => {
    if (!isLoading) {
      if (data) {
        setAuthState('authenticated')
      } else if (error) {
        setAuthState((prev) => (prev === 'mfa-challenge' ? prev : 'unauthenticated'))
      }
    }
  }, [isLoading, data, error])

  useEffect(() => {
    if (authState === 'mfa-challenge' && mfaCode.length === 6 && !mfaSubmitting) {
      handleMfaChallenge()
    }
  }, [mfaCode, authState, mfaSubmitting])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setSignInError(null)

    try {
      const res = await signIn(email, password, rememberMe, '')
      if (res.needsMfa) {
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
    return <p className="text-muted">Loading…</p>
  }

  if (authState === 'authenticated' && data) {
    const appAccess = new Set(data.appAccess ?? [])
    const isVisible = (app: AppLink) => {
      if (ALWAYS_VISIBLE.has(app.name)) return true
      if (ADMIN_ONLY.has(app.name)) return data.role === 'admin'
      return data.role === 'admin' || appAccess.has(app.accessKey ?? app.name)
    }
    const sections: AppSection[] = SECTION_DEFS.map(({ title, names }) => ({
      title,
      apps: names.map((n) => APP_MAP.get(n)!).filter((app) => isVisible(app))
    }))

    return <AppGrid sections={sections} />
  }

  if (authState === 'mfa-challenge') {
    return (
      <div className="mx-auto w-full max-w-sm">
        <h1 className="mb-4 text-center text-lg font-semibold text-fg">tools.xdoubleu.com</h1>
        <div className="rounded-2xl border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-fg">Two-factor authentication</h2>
          <div className="mt-6 space-y-4">
            <p className="text-sm text-subtle">Enter the code from your authenticator app.</p>
            <div>
              <label htmlFor="mfaChallengeCode" className="block text-sm font-medium text-subtle">
                Authenticator code
              </label>
              <Input
                id="mfaChallengeCode"
                type="text"
                inputMode="numeric"
                maxLength={6}
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                className="mt-1"
              />
            </div>
            {mfaError && (
              <p role="alert" className="text-sm text-danger">
                {mfaError}
              </p>
            )}
            <Button onClick={handleMfaChallenge} disabled={mfaSubmitting} className="w-full">
              {mfaSubmitting ? 'Verifying…' : 'Verify'}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto w-full max-w-sm">
      <h1 className="mb-4 text-center text-lg font-semibold text-fg">tools.xdoubleu.com</h1>
      <div className="rounded-2xl border border-border bg-card p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-fg">Sign In</h2>
        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-subtle">
              Email
            </label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="mt-1"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-subtle">
              Password
            </label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="mt-1"
            />
            <div className="mt-1 text-right">
              <Link href="/auth/forgot-password" className="text-sm text-accent hover:underline">
                Forgot password?
              </Link>
            </div>
          </div>

          <div className="flex items-center">
            <input
              id="rememberMe"
              type="checkbox"
              checked={rememberMe}
              onChange={(e) => setRememberMe(e.target.checked)}
              className="h-4 w-4 rounded border-input-border accent-[rgb(var(--color-accent))]"
            />
            <label htmlFor="rememberMe" className="ml-2 text-sm text-subtle">
              Remember me
            </label>
          </div>

          {signInError && (
            <p role="alert" className="text-sm text-danger">
              {signInError}
            </p>
          )}

          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  )
}
