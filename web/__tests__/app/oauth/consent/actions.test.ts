import { cookies } from 'next/headers'
import { approveAuthorization, denyAuthorization } from '@/app/oauth/consent/actions'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'

jest.mock('next/headers', () => ({ cookies: jest.fn() }))
jest.mock('@/lib/oauth2as/consentClient', () => ({
  decideAuthorization: jest.fn()
}))
jest.mock('next/navigation', () => ({
  redirect: jest.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`)
  })
}))

function mockCookies(map: Record<string, string>) {
  const store = {
    getAll: () => Object.entries(map).map(([name, value]) => ({ name, value }))
  }
  // @ts-expect-error -- partial cookie store exposing only getAll()
  jest.mocked(cookies).mockResolvedValue(store)
}

describe('consent server actions', () => {
  beforeEach(() => jest.clearAllMocks())

  it('approves and redirects to the returned url', async () => {
    mockCookies({ accessToken: 'at' })
    jest.mocked(decideAuthorization).mockResolvedValue('https://cb?code=1')

    await expect(approveAuthorization('client_id=c1&state=s1')).rejects.toThrow(
      'REDIRECT:https://cb?code=1'
    )
    expect(decideAuthorization).toHaveBeenCalledWith(
      new URLSearchParams('client_id=c1&state=s1'),
      'allow',
      'accessToken=at'
    )
  })

  it('denies and redirects to the returned url', async () => {
    mockCookies({ accessToken: 'at' })
    jest.mocked(decideAuthorization).mockResolvedValue('https://cb?error=access_denied')

    await expect(denyAuthorization('client_id=c1')).rejects.toThrow(
      'REDIRECT:https://cb?error=access_denied'
    )
    expect(decideAuthorization).toHaveBeenCalledWith(
      new URLSearchParams('client_id=c1'),
      'deny',
      'accessToken=at'
    )
  })

  it('throws when the api does not return a redirect location', async () => {
    mockCookies({ accessToken: 'at' })
    jest.mocked(decideAuthorization).mockRejectedValue(new Error('Failed to approve authorization'))
    await expect(approveAuthorization('client_id=c1')).rejects.toThrow(
      'Failed to approve authorization'
    )
  })
})
