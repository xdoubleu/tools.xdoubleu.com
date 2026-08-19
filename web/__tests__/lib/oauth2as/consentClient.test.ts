import { decideAuthorization, getConsentInfo } from '@/lib/oauth2as/consentClient'

const originalFetch = global.fetch

describe('consentClient', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.resetAllMocks()
  })

  describe('getConsentInfo', () => {
    it('returns null without a client_id', async () => {
      global.fetch = jest.fn()
      expect(await getConsentInfo(new URLSearchParams())).toBeNull()
      expect(global.fetch).not.toHaveBeenCalled()
    })

    it('returns null when the api responds with an error', async () => {
      global.fetch = jest.fn().mockResolvedValue({ ok: false })
      expect(await getConsentInfo(new URLSearchParams('client_id=c1'))).toBeNull()
    })

    it('fetches and maps the consent info response', async () => {
      const json = jest
        .fn()
        .mockResolvedValue({ client_id: 'c1', client_name: 'Claude CLI', scope: 'openid email' })
      global.fetch = jest.fn().mockResolvedValue({ ok: true, json })

      const info = await getConsentInfo(new URLSearchParams('client_id=c1&scope=openid+email'))

      expect(info).toEqual({ clientId: 'c1', clientName: 'Claude CLI', scope: 'openid email' })
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/oauth2/consent-info?client_id=c1&scope=openid+email'),
        expect.objectContaining({ cache: 'no-store' })
      )
    })
  })

  describe('decideAuthorization', () => {
    it('posts the decision and returns the redirect location', async () => {
      const headers = new Headers({ location: 'https://cb?code=1' })
      global.fetch = jest.fn().mockResolvedValue({ headers })

      const location = await decideAuthorization(
        new URLSearchParams('client_id=c1&state=s1'),
        'allow',
        'accessToken=at'
      )

      expect(location).toBe('https://cb?code=1')
      const [url, init] = jest.mocked(global.fetch).mock.calls[0]
      expect(url).toContain('/oauth2/authorize?')
      expect(url).toContain('consent=allow')
      expect(url).toContain('client_id=c1')
      expect(init).toEqual(
        expect.objectContaining({
          method: 'POST',
          redirect: 'manual',
          headers: { cookie: 'accessToken=at' }
        })
      )
    })

    it('throws when the api does not return a redirect location', async () => {
      global.fetch = jest.fn().mockResolvedValue({ headers: new Headers() })
      await expect(
        decideAuthorization(new URLSearchParams('client_id=c1'), 'deny', 'accessToken=at')
      ).rejects.toThrow('Failed to deny authorization')
    })

    it('throws an approve-specific message when denied a redirect location', async () => {
      global.fetch = jest.fn().mockResolvedValue({ headers: new Headers() })
      await expect(
        decideAuthorization(new URLSearchParams('client_id=c1'), 'allow', 'accessToken=at')
      ).rejects.toThrow('Failed to approve authorization')
    })

    it('omits the cookie header when there is no session cookie', async () => {
      const headers = new Headers({ location: 'https://cb?code=1' })
      global.fetch = jest.fn().mockResolvedValue({ headers })

      await decideAuthorization(new URLSearchParams('client_id=c1'), 'allow', '')

      const [, init] = jest.mocked(global.fetch).mock.calls[0]
      expect(init).toEqual(expect.objectContaining({ headers: undefined }))
    })
  })
})
