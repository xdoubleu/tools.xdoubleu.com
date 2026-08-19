import { render, screen } from '@testing-library/react'
import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import ConsentPage from '@/app/oauth/consent/page'
import { getConsentInfo } from '@/lib/oauth2as/consentClient'

jest.mock('next/headers', () => ({ cookies: jest.fn() }))
jest.mock('next/navigation', () => ({
  redirect: jest.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`)
  })
}))
jest.mock('@/lib/oauth2as/consentClient', () => ({ getConsentInfo: jest.fn() }))

function withCookies(map: Record<string, string>) {
  const store = {
    get: (name: string) => (name in map ? { name, value: map[name] } : undefined)
  }
  // @ts-expect-error -- partial cookie store exposing only get()
  jest.mocked(cookies).mockResolvedValue(store)
}

async function renderPage(params?: Record<string, string | string[] | undefined>) {
  const ui = await ConsentPage({ searchParams: Promise.resolve(params ?? {}) })
  render(ui)
}

describe('ConsentPage', () => {
  beforeEach(() => jest.clearAllMocks())

  it('redirects home without a client_id', async () => {
    await expect(renderPage()).rejects.toThrow('REDIRECT:/')
  })

  it('redirects to sign-in when not authenticated', async () => {
    withCookies({})
    await expect(renderPage({ client_id: 'c1' })).rejects.toThrow('REDIRECT:/auth/sign-in')
    expect(redirect).toHaveBeenCalledWith(
      expect.stringContaining(encodeURIComponent('/oauth/consent?client_id=c1'))
    )
  })

  it('renders an error when the authorization is invalid', async () => {
    withCookies({ accessToken: 'at' })
    jest.mocked(getConsentInfo).mockResolvedValue(null)
    await renderPage({ client_id: 'c1' })
    expect(
      screen.getByText('This authorization request is invalid or has expired.')
    ).toBeInTheDocument()
  })

  it('renders the consent form with client details', async () => {
    withCookies({ accessToken: 'at' })
    jest
      .mocked(getConsentInfo)
      .mockResolvedValue({ clientId: 'c1', clientName: 'Claude CLI', scope: 'openid email' })
    await renderPage({ client_id: 'c1', scope: 'openid email', state: 's1' })
    expect(screen.getByText('Authorize Claude CLI')).toBeInTheDocument()
  })

  it('takes the first value of a repeated query param', async () => {
    withCookies({ accessToken: 'at' })
    jest
      .mocked(getConsentInfo)
      .mockResolvedValue({ clientId: 'c1', clientName: 'Claude CLI', scope: 'openid' })
    await renderPage({ client_id: ['c1', 'c2'], scope: undefined })
    expect(getConsentInfo).toHaveBeenCalledWith(new URLSearchParams('client_id=c1'))
  })
})
