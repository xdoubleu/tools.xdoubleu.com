import { Code, ConnectError } from '@connectrpc/connect'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { PasskeysSection } from '@/components/settings/PasskeysSection'

const mockMutate = vi.fn()
const mockRegister = vi.fn()
const mockRename = vi.fn()
const mockRemove = vi.fn()
const mockSignOut = vi.fn()
let mockSupported = true
const mockReplace = vi.fn()

vi.mock('@/hooks/navigation', () => ({ useRouter: () => ({ replace: mockReplace }) }))

vi.mock('@/hooks/useAuth', () => ({
  passkeysSupported: () => mockSupported,
  usePasskeys: vi.fn(),
  useRegisterPasskey: () => mockRegister,
  useRenamePasskey: () => mockRename,
  useRemovePasskey: () => mockRemove,
  useSignOut: () => mockSignOut
}))

import { usePasskeys } from '@/hooks/useAuth'

const laptop = { id: 'p1', name: 'Laptop', createdAt: '2026-11-01T10:00:00Z', lastUsedAt: '' }

function list(passkeys: unknown, error?: unknown) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(usePasskeys).mockReturnValue({ data: passkeys, error, mutate: mockMutate })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockSupported = true
  mockMutate.mockResolvedValue(undefined)
  mockRegister.mockResolvedValue(undefined)
  mockRename.mockResolvedValue(undefined)
  mockRemove.mockResolvedValue(undefined)
  list([laptop])
})

describe('PasskeysSection', () => {
  it('lists passkeys with their dates, and says when there are none', () => {
    const { unmount } = render(<PasskeysSection />)
    const row = within(screen.getByRole('list', { name: 'Your passkeys' }))
    expect(row.getByText('Laptop')).toBeInTheDocument()
    expect(row.getByText(/Added/)).toBeInTheDocument()
    expect(row.queryByText(/Last used/)).not.toBeInTheDocument()
    unmount()

    list([{ ...laptop, lastUsedAt: '2026-11-02T10:00:00Z' }])
    const second = render(<PasskeysSection />)
    expect(screen.getByText(/Last used/)).toBeInTheDocument()
    second.unmount()

    list([])
    render(<PasskeysSection />)
    expect(screen.getByText('No passkeys yet.')).toBeInTheDocument()
  })

  it('adds a passkey under the typed name and refreshes the list', async () => {
    render(<PasskeysSection />)
    const add = screen.getByRole('button', { name: 'Add a passkey' })
    expect(add).toBeDisabled()

    fireEvent.change(screen.getByLabelText('Passkey name'), { target: { value: ' Phone ' } })
    fireEvent.click(add)

    await waitFor(() => expect(mockRegister).toHaveBeenCalledWith('Phone'))
    await waitFor(() => expect(mockMutate).toHaveBeenCalled())
    await waitFor(() => expect(screen.getByLabelText('Passkey name')).toHaveValue(''))
  })

  it('offers no add form when the browser lacks passkeys', () => {
    mockSupported = false
    render(<PasskeysSection />)
    expect(screen.getByText(/can't create passkeys/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add a passkey' })).not.toBeInTheDocument()
  })

  it('asks to sign in again when the sign-in is too old, then signs out and goes to sign-in', async () => {
    mockRegister.mockRejectedValue(new ConnectError('stale', Code.FailedPrecondition))
    mockSignOut.mockResolvedValue(undefined)

    render(<PasskeysSection />)
    fireEvent.change(screen.getByLabelText('Passkey name'), { target: { value: 'Phone' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add a passkey' }))

    fireEvent.click(await screen.findByRole('button', { name: 'Sign in again' }))
    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith('/auth/sign-in?next=%2Fsettings'))
    expect(mockSignOut).toHaveBeenCalled()
  })

  it('shows the server message, a cancelled prompt, or a generic failure', async () => {
    render(<PasskeysSection />)
    const type = () =>
      fireEvent.change(screen.getByLabelText('Passkey name'), { target: { value: 'Phone' } })
    const submit = () => fireEvent.click(screen.getByRole('button', { name: 'Add a passkey' }))

    mockRegister.mockRejectedValueOnce(new ConnectError('bad name', Code.InvalidArgument))
    type()
    submit()
    expect(await screen.findByText('bad name')).toBeInTheDocument()

    mockRegister.mockRejectedValueOnce(new DOMException('no', 'NotAllowedError'))
    submit()
    expect(await screen.findByText('The passkey prompt was cancelled.')).toBeInTheDocument()

    mockRegister.mockRejectedValueOnce(new Error('boom'))
    submit()
    expect(await screen.findByText('Could not add the passkey.')).toBeInTheDocument()
  })

  it('renames a passkey, and can cancel the edit', async () => {
    render(<PasskeysSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Rename Laptop' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByText('Laptop')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Rename Laptop' }))
    fireEvent.change(screen.getByLabelText('New passkey name'), {
      target: { value: 'Work laptop' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockRename).toHaveBeenCalledWith('p1', 'Work laptop'))
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Save' })).toBeNull())
  })

  it('removes a passkey only after confirming', async () => {
    render(<PasskeysSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove Laptop' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    expect(mockRemove).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Remove Laptop' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(mockRemove).toHaveBeenCalledWith('p1'))
  })

  it('says so when the list cannot be loaded', () => {
    list(undefined, new Error('down'))
    render(<PasskeysSection />)
    expect(screen.getByText('Could not load your passkeys.')).toBeInTheDocument()
  })
})
