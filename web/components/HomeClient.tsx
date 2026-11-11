import { useState, useEffect } from 'react'
import { Link } from '@/components/ui/link'
import { useCurrentUser, useSignIn, useMFAChallenge } from '@/hooks/useAuth'
import AppGrid from '@/components/AppGrid'
import { visibleSections } from '@/lib/apps'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { isSignedOut } from '@/lib/signIn'
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

export default function HomeClient() {
  const { data: lastUser, error, isLoading } = useCurrentUser()
  // SWR keeps the last user when a refetch fails; a signed-out answer means it's gone.
  const data = isSignedOut(error) ? undefined : lastUser
  const signIn = useSignIn()
  const mFAChallenge = useMFAChallenge()

  // A cached user shows the authenticated view straight away.
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
    const sections = visibleSections(data)

    // The grid's section headings carry the page; the h1 is for screen readers.
    return (
      <>
        <PageHeader title="tools.xdoubleu.com" className="sr-only" />
        <AppGrid sections={sections} />
      </>
    )
  }

  if (authState === 'mfa-challenge') {
    return (
      <PageContainer size="narrow">
        <PageHeader
          title="Two-factor authentication"
          description="Enter the code from your authenticator app."
        />
        <Card>
          <div className="space-y-4">
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
            {mfaError && <Alert tone="danger">{mfaError}</Alert>}
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
      <PageHeader title="Sign in" description="tools.xdoubleu.com" />
      <Card>
        <form onSubmit={handleSubmit} className="space-y-4">
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

          {signInError && <Alert tone="danger">{signInError}</Alert>}

          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </Card>
    </PageContainer>
  )
}
