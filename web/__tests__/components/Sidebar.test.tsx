import { create } from '@bufbuild/protobuf'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import Sidebar from '@/components/Sidebar'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn(),
  useSignOut: vi.fn(() => vi.fn())
}))
vi.mock('@/hooks/navigation', () => ({ usePathname: vi.fn() }))

import { useCurrentUser } from '@/hooks/useAuth'
import { usePathname } from '@/hooks/navigation'

function user(role: string, appAccess: string[]) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(useCurrentUser).mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role, appAccess }),
    isLoading: false,
    error: undefined
  })
}

const renderSidebar = (collapsed = false, onToggle = vi.fn()) =>
  render(
    <MemoryRouter>
      <Sidebar collapsed={collapsed} onToggle={onToggle} />
    </MemoryRouter>
  )

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(usePathname).mockReturnValue('/')
})

describe('Sidebar', () => {
  it('renders nothing while loading or signed out', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined, isLoading: true })
    expect(renderSidebar().container.firstChild).toBeNull()
  })

  it('renders nothing without a user', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined, isLoading: false })
    expect(renderSidebar().container.firstChild).toBeNull()
  })

  it('renders nothing on a shared dashboard', () => {
    user('user', [])
    vi.mocked(usePathname).mockReturnValue('/dashboard/games/some-token')
    expect(renderSidebar().container.firstChild).toBeNull()
  })

  it('lists Today and only the apps the user can open, grouped', () => {
    user('user', ['feeds', 'trains'])
    renderSidebar()
    const nav = screen.getByRole('navigation', { name: 'Apps' })
    expect(within(nav).getByRole('link', { name: 'Today' })).toHaveAttribute('href', '/')
    expect(within(nav).getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(within(nav).getByRole('link', { name: 'Trains' })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'Books' })).not.toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'Factory' })).not.toBeInTheDocument()
    expect(within(nav).getByRole('heading', { name: 'Learning' })).toBeInTheDocument()
    expect(within(nav).getByRole('heading', { name: 'Transit' })).toBeInTheDocument()
  })

  it('keeps account entries out of the app list; they live in the account menu', () => {
    user('user', [])
    renderSidebar()
    const nav = screen.getByRole('navigation', { name: 'Apps' })
    expect(within(nav).queryByRole('link', { name: 'Settings' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Account menu' })).toBeInTheDocument()
  })

  it('shows admin apps last for an admin', () => {
    user('admin', [])
    renderSidebar()
    const headings = screen.getAllByRole('heading').map((h) => h.textContent)
    expect(headings[headings.length - 1]).toBe('Admin')
    expect(screen.getByRole('link', { name: 'Factory' })).toBeInTheDocument()
  })

  it('links an external app with a plain anchor', () => {
    user('admin', [])
    renderSidebar()
    const link = screen.getByRole('link', { name: 'Grafana' })
    expect(link).toHaveAttribute('href', '/grafana')
    expect(link.tagName).toBe('A')
  })

  it('toggles collapse from the header button', () => {
    user('user', ['feeds'])
    const onToggle = vi.fn()
    renderSidebar(false, onToggle)
    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))
    expect(onToggle).toHaveBeenCalledOnce()
  })

  it('collapses to an icon rail that keeps names and the highlight', () => {
    user('user', ['feeds'])
    vi.mocked(usePathname).mockReturnValue('/feeds')
    renderSidebar(true)
    expect(screen.getByRole('button', { name: 'Expand sidebar' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
    const feeds = screen.getByRole('link', { name: 'Feeds' })
    expect(feeds).toHaveAttribute('aria-current', 'page')
    expect(screen.getByText('Feeds')).toHaveClass('sr-only')
    expect(screen.getByRole('complementary')).toHaveClass('w-16')
  })

  it('highlights Today on the home page only', () => {
    user('user', ['feeds'])
    renderSidebar()
    expect(screen.getByRole('link', { name: 'Today' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Today' })).toHaveClass('text-accent')
    expect(screen.getByRole('link', { name: 'Feeds' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('link', { name: 'Feeds' })).not.toHaveClass('text-accent')
  })

  it('highlights the app that owns the current page', () => {
    user('user', ['feeds', 'recipes'])
    vi.mocked(usePathname).mockReturnValue('/recipes/abc')
    renderSidebar()
    expect(screen.getByRole('link', { name: 'Recipes' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Recipes' })).toHaveClass('text-accent')
    expect(screen.getByRole('link', { name: 'Feeds' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('link', { name: 'Feeds' })).not.toHaveClass('text-accent')
    expect(screen.getByRole('link', { name: 'Today' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('link', { name: 'Today' })).not.toHaveClass('text-accent')
  })
})
