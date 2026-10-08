'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { useCurrentUser, useSignIn, useMFAChallenge } from '@/hooks/useAuth'
import AppGrid, { type AppLink, type AppSection } from '@/components/AppGrid'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { ConnectError } from '@connectrpc/connect'

type AuthState = 'loading' | 'authenticated' | 'unauthenticated' | 'mfa-challenge'

// Same-origin-only `next` for post-sign-in redirects. The URL parser
// collapses `//evil.com` and `/\evil.com` to their real origin.
export function safeNext(): string {
  if (typeof window === 'undefined') return '/'
  const next = new URLSearchParams(window.location.search).get('next')
  if (!next) return '/'
  try {
    const resolved = new URL(next, window.location.origin)
    if (resolved.origin === window.location.origin) {
      return resolved.pathname + resolved.search + resolved.hash
    }
  } catch {
    // fall through to the default below
  }
  return '/'
}

const ALL_APPS: AppLink[] = [
  {
    name: 'games',
    label: 'Games',
    href: '/dashboard/games',
    description: 'Steam backlog, progress and distribution.'
  },
  {
    name: 'books',
    label: 'Books',
    href: '/dashboard/reading',
    description: 'Search, library and reading progress.'
  },
  {
    name: 'feeds',
    label: 'Feeds',
    href: '/feeds',
    description: 'Unread RSS and newsletter articles.',
    accessKey: 'feeds'
  },
  {
    name: 'trains',
    label: 'Trains',
    href: '/trains',
    description: 'SNCB/NMBS station pickers and route overview.'
  },
  {
    name: 'movies',
    label: 'Movies & Series',
    href: '/movies',
    description: 'Watch backlog with TMDB search.'
  },
  {
    name: 'podcasts',
    label: 'Podcasts',
    href: '/podcasts',
    description: 'Favourite shows, found through iTunes.'
  },
  { name: 'recipes', label: 'Recipes', href: '/recipes/list', description: 'Recipe management' },
  {
    name: 'learningpaths',
    label: 'Learning Paths',
    href: '/learningpaths/list',
    description: 'Self-directed curricula with modules, items and progress tracking'
  },
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
  { name: 'settings', label: 'Settings', href: '/settings', description: 'User preferences' },
  {
    name: 'family',
    label: 'Family',
    href: '/family',
    description: 'Share one recipe book, meal plan and shopping list together'
  },
  {
    name: 'user-management',
    label: 'User management',
    href: '/user-management',
    description: 'Administration'
  },
  {
    name: 'monitoring',
    label: 'Monitoring',
    href: '/monitoring/connections',
    description: 'Observability'
  },
  {
    name: 'observability',
    label: 'Automated actions',
    href: '/monitoring/observability',
    description: 'Self-healing routine run history'
  },
  {
    name: 'grafana',
    label: 'Grafana',
    href: '/grafana',
    description: 'Metrics dashboards and alerting',
    external: true
  }
]

const APP_MAP = new Map(ALL_APPS.map((a) => [a.name, a]))

const SECTION_DEFS: { title: string; names: string[] }[] = [
  {
    title: 'Productivity',
    names: ['games', 'books', 'feeds', 'movies', 'podcasts', 'trains', 'learningpaths']
  },
  { title: 'Food', names: ['recipes', 'mealplans', 'shoppinglist'] },
  { title: 'Tools', names: ['watchparty'] },
  { title: 'Account', names: ['settings', 'family'] },
  { title: 'Admin', names: ['user-management', 'monitoring', 'observability', 'grafana'] }
]

const ALWAYS_VISIBLE = new Set(['settings', 'family'])
const ADMIN_ONLY = new Set(['user-management', 'monitoring', 'observability', 'grafana'])

export default function HomeClient() {
  const { data, error, isLoading } = useCurrentUser()
  const signIn = useSignIn()
  const mFAChallenge = useMFAChallenge()

  // SWRProvider's fallback lets the authenticated view server-render.
  const [authState, setAuthState] = useState<AuthState>(data ? 'authenticated' : 'loading')
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
        if (
          typeof window !== 'undefined' &&
          new URLSearchParams(window.location.search).has('next')
        ) {
          window.location.href = safeNext()
          return
        }
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
      } else if (typeof window !== 'undefined') {
        window.location.href = safeNext()
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
        window.location.href = safeNext()
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
    return <LoadingState />
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
      <PageContainer size="narrow">
        <PageHeader title="tools.xdoubleu.com" />
        <Card className="p-6">
          <h2 className="text-lg font-semibold text-fg">Two-factor authentication</h2>
          <div className="mt-6 space-y-4">
            <p className="text-sm text-subtle">Enter the code from your authenticator app.</p>
            <Field label="Authenticator code" htmlFor="mfaChallengeCode">
              <Input
                id="mfaChallengeCode"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
              />
            </Field>
            {mfaError && (
              <p role="alert" className="text-sm text-danger">
                {mfaError}
              </p>
            )}
            <Button onClick={handleMfaChallenge} disabled={mfaSubmitting} className="w-full">
              {mfaSubmitting ? 'Verifying…' : 'Verify'}
            </Button>
          </div>
        </Card>
      </PageContainer>
    )
  }

  return (
    <PageContainer size="narrow">
      <PageHeader title="tools.xdoubleu.com" />
      <Card className="p-6">
        <h2 className="text-lg font-semibold text-fg">Sign In</h2>
        <form onSubmit={handleSubmit} className="mt-6 space-y-4">
          <Field label="Email" htmlFor="email">
            <Input
              id="email"
              type="email"
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </Field>

          <div>
            <Field label="Password" htmlFor="password">
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </Field>
            <div className="mt-2 text-right">
              <Button asChild variant="link">
                <Link href="/auth/forgot-password">Forgot password?</Link>
              </Button>
            </div>
          </div>

          <Checkbox
            id="rememberMe"
            label="Remember me"
            checked={rememberMe}
            onChange={(e) => setRememberMe(e.target.checked)}
          />

          {signInError && (
            <p role="alert" className="text-sm text-danger">
              {signInError}
            </p>
          )}

          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </Card>
    </PageContainer>
  )
}
