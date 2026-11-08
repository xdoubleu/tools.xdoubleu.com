import { fireEvent, render, screen } from '@testing-library/react'
import DashboardSwitch from '@/components/dashboard/DashboardSwitch'
import { useRouter } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'

vi.mock('@/hooks/useAuth', () => ({ useCurrentUser: vi.fn() }))

function mockUser(data: { role: string; appAccess: string[] }) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only data is read
  vi.mocked(useCurrentUser).mockReturnValue({ data } as never)
}

describe('DashboardSwitch', () => {
  beforeEach(() => vi.mocked(useRouter().push).mockClear())

  it('navigates to the other dashboard', () => {
    mockUser({ role: 'user', appAccess: ['books', 'games'] })
    render(<DashboardSwitch current="reading" />)
    expect(screen.getByRole('tab', { name: 'Books' })).toHaveAttribute('aria-selected', 'true')
    fireEvent.click(screen.getByRole('tab', { name: 'Games' }))
    expect(useRouter().push).toHaveBeenCalledWith('/dashboard/games')
  })

  it('stays put on the current dashboard', () => {
    mockUser({ role: 'admin', appAccess: [] })
    render(<DashboardSwitch current="games" />)
    fireEvent.click(screen.getByRole('tab', { name: 'Games' }))
    expect(useRouter().push).not.toHaveBeenCalled()
  })

  it('hides without access to both', () => {
    mockUser({ role: 'user', appAccess: ['books'] })
    const { container } = render(<DashboardSwitch current="reading" />)
    expect(container).toBeEmptyDOMElement()
  })
})
