import { approveAuthorization, denyAuthorization } from '@/app/oauth/consent/actions'
import { createOAuthServerClient } from '@/lib/supabase/oauthServer'

jest.mock('@/lib/supabase/oauthServer', () => ({
  createOAuthServerClient: jest.fn()
}))

jest.mock('next/navigation', () => ({
  redirect: jest.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`)
  })
}))

function mockClient(oauth: Record<string, unknown>) {
  const client = { auth: { oauth } }
  // @ts-expect-error -- partial supabase client for the oauth namespace only
  jest.mocked(createOAuthServerClient).mockResolvedValue(client)
}

describe('consent server actions', () => {
  beforeEach(() => jest.clearAllMocks())

  it('approves and redirects to the returned url', async () => {
    const approve = jest.fn().mockResolvedValue({ data: { redirect_url: 'https://cb?code=1' } })
    mockClient({ approveAuthorization: approve })

    await expect(approveAuthorization('auth-1')).rejects.toThrow('REDIRECT:https://cb?code=1')
    expect(approve).toHaveBeenCalledWith('auth-1', { skipBrowserRedirect: true })
  })

  it('denies and redirects to the returned url', async () => {
    const deny = jest.fn().mockResolvedValue({ data: { redirect_url: 'https://cb?error=denied' } })
    mockClient({ denyAuthorization: deny })

    await expect(denyAuthorization('auth-1')).rejects.toThrow('REDIRECT:https://cb?error=denied')
    expect(deny).toHaveBeenCalledWith('auth-1', { skipBrowserRedirect: true })
  })

  it('throws when the client is not configured', async () => {
    jest.mocked(createOAuthServerClient).mockResolvedValue(null)
    await expect(approveAuthorization('auth-1')).rejects.toThrow('not configured')
  })

  it('throws when Supabase returns an error', async () => {
    const approve = jest.fn().mockResolvedValue({ error: { message: 'nope' } })
    mockClient({ approveAuthorization: approve })
    await expect(approveAuthorization('auth-1')).rejects.toThrow('nope')
  })
})
