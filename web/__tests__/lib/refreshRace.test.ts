import { fetchSurvivingRefreshRace } from '@/lib/refreshRace'

const userUrl = 'https://api.test/auth.v1.AuthService/GetCurrentUser'
const signInUrl = 'https://api.test/auth.v1.AuthService/SignIn'

const status = (code: number) => new Response(null, { status: code })

function deferred() {
  let resolve!: (res: Response) => void
  const promise = new Promise<Response>((r) => (resolve = r))
  return { promise, resolve }
}

describe('fetchSurvivingRefreshRace without Web Locks', () => {
  it('retries a lost race only after the request that rotated the session settles', async () => {
    const winner = deferred()
    const events: string[] = []
    void fetchSurvivingRefreshRace(userUrl, () => winner.promise).then(() => events.push('winner'))

    const loser = vi
      .fn<() => Promise<Response>>()
      .mockResolvedValueOnce(status(401))
      .mockImplementationOnce(async () => {
        events.push('retry')
        return status(200)
      })
    const result = fetchSurvivingRefreshRace(userUrl, loser)

    await vi.waitFor(() => expect(loser).toHaveBeenCalledTimes(1))
    expect(events).toEqual([])
    winner.resolve(status(200))

    expect((await result).status).toBe(200)
    expect(events).toEqual(['winner', 'retry'])
  })

  it('surfaces a real sign-out after one retry', async () => {
    const send = vi.fn(async () => status(401))
    expect((await fetchSurvivingRefreshRace(userUrl, send)).status).toBe(401)
    expect(send).toHaveBeenCalledTimes(2)
  })

  it('does not retry a credential check', async () => {
    const send = vi.fn(async () => status(401))
    expect((await fetchSurvivingRefreshRace(signInUrl, send)).status).toBe(401)
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('recognises a credential check sent as a Request or URL', async () => {
    const send = vi.fn(async () => status(401))
    await fetchSurvivingRefreshRace(new Request(signInUrl), send)
    await fetchSurvivingRefreshRace(new URL(signInUrl), send)
    expect(send).toHaveBeenCalledTimes(2)
  })

  it('does not retry other responses', async () => {
    const send = vi.fn(async () => status(403))
    expect((await fetchSurvivingRefreshRace(userUrl, send)).status).toBe(403)
    expect(send).toHaveBeenCalledTimes(1)
  })
})

describe('fetchSurvivingRefreshRace with Web Locks', () => {
  const modes: string[] = []

  beforeEach(() => {
    modes.length = 0
    vi.stubGlobal('navigator', {
      locks: {
        request: (_name: string, options: { mode: string }, callback: () => Promise<Response>) => {
          modes.push(options.mode)
          return callback()
        }
      }
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends under the shared lock and retries under the exclusive one', async () => {
    const send = vi
      .fn<() => Promise<Response>>()
      .mockResolvedValueOnce(status(401))
      .mockResolvedValueOnce(status(200))
    expect((await fetchSurvivingRefreshRace(userUrl, send)).status).toBe(200)
    expect(modes).toEqual(['shared', 'exclusive'])
  })

  it('takes only the shared lock when the first answer stands', async () => {
    await fetchSurvivingRefreshRace(userUrl, async () => status(200))
    expect(modes).toEqual(['shared'])
  })
})
