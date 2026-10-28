import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import ConsentPage from '@/app/oauth/consent/page'
import { useCurrentUser } from '@/hooks/useAuth'
import { useSearchParams } from '@/hooks/navigation'
import { getConsentInfo } from '@/lib/oauth2as/consentClient'

vi.mock('@/hooks/useAuth', () => ({ useCurrentUser: vi.fn() }))
vi.mock('@/lib/oauth2as/consentClient', () => ({ getConsentInfo: vi.fn() }))
vi.mock('@/app/oauth/consent/ConsentForm', () => ({
  default: (props: { requestQuery: string; consentToken: string; clientName: string }) => (
    <div data-testid="form">{`${props.requestQuery}|${props.consentToken}|${props.clientName}`}</div>
  )
}))

function Where() {
  const location = useLocation()
  return <div data-testid="where">{location.pathname + location.search}</div>
}

function renderPage() {
  render(
    <MemoryRouter>
      <ConsentPage />
      <Routes>
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>
  )
}

function signedIn(signed: boolean, isLoading = false) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only data and isLoading are read
  vi.mocked(useCurrentUser).mockReturnValue({
    data: signed ? { userId: 'u1' } : undefined,
    isLoading
  } as never)
}

describe('ConsentPage', () => {
  beforeEach(() => {
    vi.mocked(getConsentInfo).mockReset()
    vi.mocked(useSearchParams).mockReturnValue(
      new URLSearchParams('client_id=c1&scope=openid&state=s1')
    )
  })

  it('goes home without a client_id', () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams())
    signedIn(true)
    renderPage()

    expect(screen.getByTestId('where')).toHaveTextContent('/')
    expect(getConsentInfo).not.toHaveBeenCalled()
  })

  it('waits for the session before deciding anything', () => {
    signedIn(false, true)
    renderPage()

    expect(screen.getByText(/loading/i)).toBeInTheDocument()
    expect(getConsentInfo).not.toHaveBeenCalled()
  })

  it('sends a signed-out user to sign in and back', () => {
    signedIn(false)
    renderPage()

    const next = encodeURIComponent('/oauth/consent?client_id=c1&scope=openid&state=s1')
    expect(screen.getByTestId('where')).toHaveTextContent(`/auth/sign-in?next=${next}`)
  })

  it('shows the consent form with the echoed request and its token', async () => {
    signedIn(true)
    vi.mocked(getConsentInfo).mockResolvedValue({
      clientId: 'c1',
      clientName: 'Claude CLI',
      scope: 'openid',
      consentToken: 'tok'
    })
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId('form')).toHaveTextContent(
        'client_id=c1&scope=openid&state=s1|tok|Claude CLI'
      )
    )
  })

  it('reports an invalid or expired request', async () => {
    signedIn(true)
    vi.mocked(getConsentInfo).mockResolvedValue(null)
    renderPage()

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid or has expired/)
    )
  })

  it('treats a failed lookup as an invalid request', async () => {
    signedIn(true)
    vi.mocked(getConsentInfo).mockRejectedValue(new Error('network'))
    renderPage()

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid or has expired/)
    )
  })
})
