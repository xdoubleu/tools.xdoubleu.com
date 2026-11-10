import { decideAuthorization, getConsentInfo } from '@/lib/oauth2as/consentClient'

const originalFetch = global.fetch

function jsonResponse(body: unknown, ok = true) {
  return { ok, json: vi.fn().mockResolvedValue(body) }
}

describe('consentClient', () => {
  afterEach(() => {
    global.fetch = originalFetch
    vi.resetAllMocks()
  })

  describe('getConsentInfo', () => {
    it('returns null without a client_id', async () => {
      global.fetch = vi.fn()
      expect(await getConsentInfo(new URLSearchParams())).toBeNull()
      expect(global.fetch).not.toHaveBeenCalled()
    })

    it('returns null when the api responds with an error', async () => {
      global.fetch = vi.fn().mockResolvedValue({ ok: false })
      expect(await getConsentInfo(new URLSearchParams('client_id=c1'))).toBeNull()
    })

    it('fetches the full request with the session and maps the response', async () => {
      global.fetch = vi.fn().mockResolvedValue(
        jsonResponse({
          client_id: 'c1',
          client_name: 'Claude CLI',
          scope: 'openid email',
          consent_token: 'tok',
          can_delegate: true
        })
      )

      const info = await getConsentInfo(
        new URLSearchParams('client_id=c1&scope=openid+email&state=s1')
      )

      expect(info).toEqual({
        clientId: 'c1',
        clientName: 'Claude CLI',
        scope: 'openid email',
        consentToken: 'tok',
        canDelegate: true
      })
      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining('/oauth2/consent-info?client_id=c1&scope=openid+email&state=s1'),
        expect.objectContaining({ cache: 'no-store', credentials: 'include' })
      )
    })

    it('has no consent token without a session', async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(jsonResponse({ client_id: 'c1', client_name: 'X', scope: '' }))

      const info = await getConsentInfo(new URLSearchParams('client_id=c1'))

      expect(info?.consentToken).toBeUndefined()
      expect(info?.canDelegate).toBe(false)
    })
  })

  describe('decideAuthorization', () => {
    it('names the agent account an approval delegates to', async () => {
      global.fetch = vi.fn().mockResolvedValue(jsonResponse({ location: 'https://cb?code=1' }))

      await decideAuthorization(new URLSearchParams('client_id=c1'), 'allow', 'tok', 'acct-1')

      const [url] = vi.mocked(global.fetch).mock.calls[0]
      expect(url).toContain('agent_account_id=acct-1')
    })

    it('posts the decision and returns the redirect location', async () => {
      global.fetch = vi.fn().mockResolvedValue(jsonResponse({ location: 'https://cb?code=1' }))

      const location = await decideAuthorization(
        new URLSearchParams('client_id=c1&state=s1'),
        'allow',
        'tok'
      )

      expect(location).toBe('https://cb?code=1')
      const [url, init] = vi.mocked(global.fetch).mock.calls[0]
      expect(url).toContain('/oauth2/authorize?')
      expect(url).toContain('consent=allow')
      expect(url).toContain('client_id=c1')
      expect(init).toEqual(
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
          headers: { Accept: 'application/json', 'X-OAuth-Consent-Token': 'tok' }
        })
      )
    })

    it('sends no consent token when denying', async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(jsonResponse({ location: 'https://cb?error=denied' }))

      await decideAuthorization(new URLSearchParams('client_id=c1'), 'deny')

      const [, init] = vi.mocked(global.fetch).mock.calls[0]
      expect(init).toEqual(expect.objectContaining({ headers: { Accept: 'application/json' } }))
    })

    it('throws when the api does not return a redirect location', async () => {
      global.fetch = vi.fn().mockResolvedValue(jsonResponse({}))
      await expect(
        decideAuthorization(new URLSearchParams('client_id=c1'), 'deny')
      ).rejects.toThrow('Failed to deny authorization')
    })

    it('throws an approve-specific message when approval returns no location', async () => {
      global.fetch = vi.fn().mockResolvedValue({
        ok: false,
        json: vi.fn().mockRejectedValue(new SyntaxError('not json'))
      })
      await expect(
        decideAuthorization(new URLSearchParams('client_id=c1'), 'allow', 'tok')
      ).rejects.toThrow('Failed to approve authorization')
    })
  })
})
