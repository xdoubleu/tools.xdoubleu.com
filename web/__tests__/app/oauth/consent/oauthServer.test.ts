import { cookies } from 'next/headers'
import { createClient } from '@supabase/supabase-js'
import { createOAuthServerClient } from '@/lib/supabase/oauthServer'

jest.mock('next/headers', () => ({ cookies: jest.fn() }))
jest.mock('@supabase/supabase-js', () => ({ createClient: jest.fn() }))

function mockCookies(map: Record<string, string>) {
  const store = {
    get: (name: string) => (name in map ? { name, value: map[name] } : undefined)
  }
  // @ts-expect-error -- partial cookie store exposing only get()
  jest.mocked(cookies).mockResolvedValue(store)
}

describe('createOAuthServerClient', () => {
  const original = { url: process.env.SUPABASE_URL, key: process.env.SUPABASE_ANON_KEY }

  beforeEach(() => {
    jest.clearAllMocks()
    process.env.SUPABASE_URL = 'https://ref.supabase.co'
    process.env.SUPABASE_ANON_KEY = 'anon'
  })

  afterAll(() => {
    process.env.SUPABASE_URL = original.url
    process.env.SUPABASE_ANON_KEY = original.key
  })

  it('returns null when env is not configured', async () => {
    delete process.env.SUPABASE_URL
    expect(await createOAuthServerClient()).toBeNull()
    expect(createClient).not.toHaveBeenCalled()
  })

  it('returns null when there is no access token cookie', async () => {
    mockCookies({})
    expect(await createOAuthServerClient()).toBeNull()
  })

  it('builds a client and sets the session from cookies', async () => {
    mockCookies({ accessToken: 'at', refreshToken: 'rt' })
    const setSession = jest.fn().mockResolvedValue({ data: {}, error: null })
    const fakeClient = { auth: { setSession } }
    // @ts-expect-error -- partial supabase client exposing only auth.setSession
    jest.mocked(createClient).mockReturnValue(fakeClient)

    const client = await createOAuthServerClient()
    expect(client).not.toBeNull()
    expect(createClient).toHaveBeenCalledWith(
      'https://ref.supabase.co',
      'anon',
      expect.objectContaining({ auth: expect.objectContaining({ persistSession: false }) })
    )
    expect(setSession).toHaveBeenCalledWith({ access_token: 'at', refresh_token: 'rt' })
  })
})
