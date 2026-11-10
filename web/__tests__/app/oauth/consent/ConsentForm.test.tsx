import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'
import ConsentForm from '@/app/oauth/consent/ConsentForm'
import { leaveTo } from '@/lib/navigate'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'
import { useAgents } from '@/hooks/useUserManagement'
import {
  AgentAccountSchema,
  ListAgentAccountsResponseSchema
} from '@/lib/gen/factory/v1/agent_accounts_pb'

const mockCreate = vi.fn()
const mockSetAccess = vi.fn()
vi.mock('@/lib/oauth2as/consentClient', () => ({ decideAuthorization: vi.fn() }))
vi.mock('@/lib/navigate', () => ({ leaveTo: vi.fn(), reloadPage: vi.fn() }))
vi.mock('@/hooks/useUserManagement', () => ({
  useAgents: vi.fn(),
  useAgentActions: vi.fn(() => ({ create: mockCreate, setAccess: mockSetAccess }))
}))

const agents = create(ListAgentAccountsResponseSchema, {
  accounts: [
    create(AgentAccountSchema, { id: 'a1', name: 'Reviewer' }),
    create(AgentAccountSchema, { id: 'a2', name: 'Gone', revokedAt: '2026-10-01T00:00:00Z' })
  ]
})

function renderDelegating() {
  render(
    <ConsentForm
      consentToken="tok"
      requestQuery="client_id=c1"
      clientName="Claude"
      scope="openid"
      canDelegate
    />
  )
}

function lastDelegation() {
  const calls = vi.mocked(decideAuthorization).mock.calls
  return calls[calls.length - 1][3]
}

describe('ConsentForm', () => {
  beforeEach(() => {
    vi.mocked(decideAuthorization).mockReset().mockResolvedValue('https://cb?code=1')
    vi.mocked(leaveTo).mockReset()
    mockCreate.mockReset().mockResolvedValue({
      account: create(AgentAccountSchema, { id: 'new-1', userId: 'u-new' }),
      clientSecret: 's'
    })
    mockSetAccess.mockReset().mockResolvedValue(undefined)
    // @ts-expect-error -- partial SWRResponse
    vi.mocked(useAgents).mockReturnValue({ data: agents })
  })

  it('offers no delegation to a person', () => {
    render(<ConsentForm consentToken="tok" requestQuery="client_id=c1" clientName="X" scope="" />)
    expect(screen.queryByRole('radiogroup', { name: 'Sign in as' })).not.toBeInTheDocument()
  })

  it('signs an admin in as themselves by default', async () => {
    renderDelegating()
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    expect(lastDelegation()).toBeUndefined()
    expect(mockCreate).not.toHaveBeenCalled()
  })

  it('delegates to an existing active agent', async () => {
    renderDelegating()
    fireEvent.click(screen.getByLabelText('Delegate to an existing agent'))
    const approve = screen.getByRole('button', { name: 'Approve' })
    expect(approve).toBeDisabled()
    expect(screen.queryByRole('option', { name: 'Gone' })).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Agent'), { target: { value: 'a1' } })
    fireEvent.click(approve)

    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    expect(lastDelegation()).toBe('a1')
  })

  it('creates a new agent with the chosen grants, then delegates to it', async () => {
    renderDelegating()
    fireEvent.click(screen.getByLabelText('Delegate to a new agent'))
    expect(screen.getByLabelText('Agent name')).toHaveValue('Claude')
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled()

    fireEvent.change(screen.getByLabelText('books access for the new agent'), {
      target: { value: 'read' }
    })
    fireEvent.change(screen.getByLabelText('factory access for the new agent'), {
      target: { value: 'none' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    expect(mockCreate).toHaveBeenCalledWith('Claude')
    expect(mockSetAccess.mock.calls).toEqual([
      ['u-new', 'factory', 'none'],
      ['u-new', 'books', 'read']
    ])
    expect(lastDelegation()).toBe('new-1')
  })

  it('names the new agent as typed and ignores an unknown grant level', async () => {
    renderDelegating()
    fireEvent.click(screen.getByLabelText('Delegate to a new agent'))
    fireEvent.change(screen.getByLabelText('Agent name'), { target: { value: ' Night shift ' } })
    fireEvent.change(screen.getByLabelText('books access for the new agent'), {
      target: { value: 'admin' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    expect(mockCreate).toHaveBeenCalledWith('Night shift')
    expect(mockSetAccess).not.toHaveBeenCalled()
  })

  it('says when the agents could not be loaded', () => {
    // @ts-expect-error -- partial SWRResponse
    vi.mocked(useAgents).mockReturnValue({ error: new Error('down') })
    renderDelegating()
    expect(screen.getByText("Couldn't load your agents.")).toBeInTheDocument()
    expect(screen.getByLabelText('Delegate to an existing agent')).toBeDisabled()
  })

  it('shows why creating the agent failed', async () => {
    mockCreate.mockRejectedValue(new ConnectError('nope', Code.ResourceExhausted))
    renderDelegating()
    fireEvent.click(screen.getByLabelText('Delegate to a new agent'))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() => expect(screen.getByText(/Too many attempts/)).toBeInTheDocument())
    expect(decideAuthorization).not.toHaveBeenCalled()
  })

  it('reuses the agent it made when the approval is retried', async () => {
    vi.mocked(decideAuthorization).mockRejectedValueOnce(new Error('boom'))
    mockCreate.mockResolvedValueOnce({ clientSecret: 's' })
    renderDelegating()
    fireEvent.click(screen.getByLabelText('Delegate to a new agent'))
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() =>
      expect(screen.getByText("Couldn't create the agent. Try again.")).toBeInTheDocument()
    )

    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(screen.getByText(/Something went wrong/)).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    expect(mockCreate).toHaveBeenCalledTimes(2)
    expect(lastDelegation()).toBe('new-1')
  })

  it('renders the client name and human-readable scopes', () => {
    render(
      <ConsentForm
        consentToken="tok"
        requestQuery="client_id=c1"
        clientName="Claude CLI"
        scope="openid email"
      />
    )
    expect(screen.getByText('Authorize Claude CLI')).toBeInTheDocument()
    expect(screen.getByText('Verify your identity')).toBeInTheDocument()
    expect(screen.getByText('Read your email address')).toBeInTheDocument()
  })

  it('shows unknown scopes verbatim', () => {
    render(
      <ConsentForm
        consentToken="tok"
        requestQuery="client_id=c1"
        clientName="X"
        scope="custom:scope"
      />
    )
    expect(screen.getByText('custom:scope')).toBeInTheDocument()
  })

  it('approves with the original request query and consent token, then follows the redirect', async () => {
    render(
      <ConsentForm
        consentToken="tok"
        requestQuery="client_id=c1&scope=openid"
        clientName="X"
        scope="openid"
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() => expect(leaveTo).toHaveBeenCalledWith('https://cb?code=1'))
    const [query, decision, token] = vi.mocked(decideAuthorization).mock.calls[0]
    expect(query.toString()).toBe('client_id=c1&scope=openid')
    expect([decision, token]).toEqual(['allow', 'tok'])
  })

  it('denies with the original request query and no consent token', async () => {
    render(
      <ConsentForm
        consentToken="tok"
        requestQuery="client_id=c1&scope=openid"
        clientName="X"
        scope="openid"
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))

    await waitFor(() => expect(leaveTo).toHaveBeenCalled())
    const [query, decision, token] = vi.mocked(decideAuthorization).mock.calls[0]
    expect(query.toString()).toBe('client_id=c1&scope=openid')
    expect([decision, token]).toEqual(['deny', undefined])
  })

  it('surfaces an error and re-enables the buttons when the decision fails', async () => {
    vi.mocked(decideAuthorization).mockRejectedValue(new Error('boom'))
    render(
      <ConsentForm consentToken="tok" requestQuery="client_id=c1" clientName="X" scope="openid" />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))

    await waitFor(() =>
      expect(screen.getByText('Something went wrong. Please try again.')).toBeInTheDocument()
    )
    expect(screen.getByRole('button', { name: 'Approve' })).toBeEnabled()
    expect(leaveTo).not.toHaveBeenCalled()
  })
})
