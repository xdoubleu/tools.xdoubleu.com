import { render, screen } from '@testing-library/react'
import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import ConsentPage from '@/app/oauth/consent/page'
import { createOAuthServerClient } from '@/lib/supabase/oauthServer'

jest.mock('next/headers', () => ({ cookies: jest.fn() }))
jest.mock('next/navigation', () => ({
  redirect: jest.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`)
  })
}))
jest.mock('@/lib/supabase/oauthServer', () => ({ createOAuthServerClient: jest.fn() }))

function withCookies(map: Record<string, string>) {
  const store = {
    get: (name: string) => (name in map ? { name, value: map[name] } : undefined)
  }
  // @ts-expect-error -- partial cookie store exposing only get()
  jest.mocked(cookies).mockResolvedValue(store)
}

function supabaseReturning(result: unknown) {
  const getAuthorizationDetails = jest.fn().mockResolvedValue(result)
  const client = { auth: { oauth: { getAuthorizationDetails } } }
  // @ts-expect-error -- partial supabase client exposing only the oauth namespace
  jest.mocked(createOAuthServerClient).mockResolvedValue(client)
}

async function renderPage(authorizationId?: string) {
  const ui = await ConsentPage({
    searchParams: Promise.resolve(authorizationId ? { authorization_id: authorizationId } : {})
  })
  render(ui)
}

describe('ConsentPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('redirects home without an authorization id', async () => {
    await expect(renderPage()).rejects.toThrow('REDIRECT:/')
  })

  it('redirects to sign-in when not authenticated', async () => {
    withCookies({})
    await expect(renderPage('auth-1')).rejects.toThrow('REDIRECT:/auth/sign-in')
    expect(redirect).toHaveBeenCalledWith(
      expect.stringContaining(encodeURIComponent('/oauth/consent?authorization_id=auth-1'))
    )
  })

  it('renders a config error when the client is unavailable', async () => {
    withCookies({ accessToken: 'at' })
    jest.mocked(createOAuthServerClient).mockResolvedValue(null)
    await renderPage('auth-1')
    expect(screen.getByText('OAuth server is not configured.')).toBeInTheDocument()
  })

  it('renders an error when the authorization is invalid', async () => {
    withCookies({ accessToken: 'at' })
    supabaseReturning({ error: { message: 'bad' } })
    await renderPage('auth-1')
    expect(
      screen.getByText('This authorization request is invalid or has expired.')
    ).toBeInTheDocument()
  })

  it('redirects immediately when consent already granted', async () => {
    withCookies({ accessToken: 'at' })
    supabaseReturning({ data: { redirect_url: 'https://cb?code=1' } })
    await expect(renderPage('auth-1')).rejects.toThrow('REDIRECT:https://cb?code=1')
  })

  it('renders the consent form with client details', async () => {
    withCookies({ accessToken: 'at' })
    supabaseReturning({
      data: {
        authorization_id: 'auth-1',
        client: { name: 'Claude CLI' },
        scope: 'openid email'
      }
    })
    await renderPage('auth-1')
    expect(screen.getByText('Authorize Claude CLI')).toBeInTheDocument()
  })
})
