import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import AccountMenu from '@/components/AccountMenu'

vi.mock('@/hooks/useAuth', () => ({ useSignOut: vi.fn() }))
vi.mock('@/lib/offline/session', () => ({ clearOfflineData: vi.fn(async () => {}) }))
vi.mock('@/lib/offline/outbox', () => ({ flushOutbox: vi.fn(async () => {}) }))

import { useSignOut } from '@/hooks/useAuth'
import { clearOfflineData } from '@/lib/offline/session'
import { flushOutbox } from '@/lib/offline/outbox'

const mockUseSignOut = vi.mocked(useSignOut)

function openMenu() {
  render(
    <MemoryRouter>
      <AccountMenu />
    </MemoryRouter>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Account menu' }))
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  mockUseSignOut.mockReturnValue(vi.fn().mockResolvedValue({}))
})

describe('AccountMenu', () => {
  it('stays closed until the button is pressed', () => {
    render(
      <MemoryRouter>
        <AccountMenu />
      </MemoryRouter>
    )
    expect(screen.queryByRole('menuitem')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Account menu' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })

  it('links to account settings and family', () => {
    openMenu()
    expect(screen.getByRole('menuitem', { name: 'Account settings' })).toHaveAttribute(
      'href',
      '/settings'
    )
    expect(screen.getByRole('menuitem', { name: 'Family' })).toHaveAttribute('href', '/family')
  })

  it.each(['Account settings', 'Family'])('closes after following %s', (name) => {
    openMenu()
    fireEvent.click(screen.getByRole('menuitem', { name }))
    expect(screen.queryByRole('menuitem', { name })).not.toBeInTheDocument()
  })

  it('marks the stored theme and switches it', () => {
    localStorage.setItem('theme', 'light')
    openMenu()
    expect(screen.getByRole('menuitemradio', { name: 'Light' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByRole('menuitemradio', { name: 'Dark' })).toHaveAttribute(
      'aria-checked',
      'false'
    )

    fireEvent.click(screen.getByRole('menuitemradio', { name: 'Dark' }))

    expect(localStorage.getItem('theme')).toBe('dark')
    expect(screen.getByRole('menuitemradio', { name: 'Dark' })).toHaveClass('text-accent')
    expect(screen.getByRole('menuitemradio', { name: 'Light' })).not.toHaveClass('text-accent')
  })

  it('sends queued writes, signs out, then clears offline data', async () => {
    const signOut = vi.fn().mockResolvedValue({})
    mockUseSignOut.mockReturnValue(signOut)
    openMenu()

    fireEvent.click(screen.getByRole('menuitem', { name: 'Sign out' }))

    await waitFor(() => expect(clearOfflineData).toHaveBeenCalledTimes(1))
    expect(signOut).toHaveBeenCalledTimes(1)
    expect(vi.mocked(flushOutbox).mock.invocationCallOrder[0]).toBeLessThan(
      signOut.mock.invocationCallOrder[0]
    )
    expect(signOut.mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(clearOfflineData).mock.invocationCallOrder[0]
    )
  })
})
