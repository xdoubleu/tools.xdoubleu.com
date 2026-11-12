import { create } from '@bufbuild/protobuf'
import { render, renderHook, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import PhoneTabBar, { useTabBarVisible } from '@/components/PhoneTabBar'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

vi.mock('@/hooks/useAuth', () => ({ useCurrentUser: vi.fn() }))
vi.mock('@/hooks/navigation', () => ({ usePathname: vi.fn() }))

import { useCurrentUser } from '@/hooks/useAuth'
import { usePathname } from '@/hooks/navigation'

function user(role: string, appAccess: string[]) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(useCurrentUser).mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role, appAccess })
  })
}

const renderBar = (pathname: string) => {
  vi.mocked(usePathname).mockReturnValue(pathname)
  return render(
    <MemoryRouter>
      <PhoneTabBar />
    </MemoryRouter>
  )
}

const labels = () => screen.getAllByRole('link').map((l) => l.textContent)
const current = () => screen.getByRole('link', { current: 'page' }).textContent

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PhoneTabBar', () => {
  it('renders nothing signed out', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined })
    expect(renderBar('/').container.firstChild).toBeNull()
  })

  it.each(['/books/1/read', '/watchparty/abc/presenter', '/dashboard/games/some-token'])(
    'renders nothing on %s',
    (path) => {
      user('admin', [])
      expect(renderBar(path).container.firstChild).toBeNull()
    }
  )

  it('shows Today, the pinned apps and Apps', () => {
    user('user', ['books', 'feeds', 'mealplans', 'trains'])
    renderBar('/')
    expect(labels()).toEqual(['Today', 'Books', 'Feeds', 'Meal plans', 'Apps'])
    expect(screen.getByRole('link', { name: 'Apps' })).toHaveAttribute('href', '/apps')
    expect(screen.getByRole('link', { name: 'Today' })).toHaveAttribute('href', '/')
  })

  it('has fewer slots for a user with fewer apps', () => {
    user('user', ['trains'])
    renderBar('/')
    expect(labels()).toEqual(['Today', 'Trains', 'Apps'])
  })

  it('highlights Today on the home page only', () => {
    user('user', ['books'])
    renderBar('/')
    expect(current()).toBe('Today')
  })

  it('highlights the pinned app that owns the page', () => {
    user('user', ['books', 'feeds'])
    renderBar('/feeds')
    expect(current()).toBe('Feeds')
  })

  it.each(['/apps', '/trains', '/settings', '/nowhere'])('highlights Apps on %s', (path) => {
    user('user', ['books', 'feeds', 'mealplans', 'trains'])
    renderBar(path)
    expect(current()).toBe('Apps')
  })
})

describe('useTabBarVisible', () => {
  it('is true for a signed-in user on an ordinary page', () => {
    user('user', [])
    vi.mocked(usePathname).mockReturnValue('/feeds')
    expect(renderHook(() => useTabBarVisible()).result.current).toBe(true)
  })

  it('is false signed out or on a full-screen page', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined })
    vi.mocked(usePathname).mockReturnValue('/feeds')
    expect(renderHook(() => useTabBarVisible()).result.current).toBe(false)

    user('user', [])
    vi.mocked(usePathname).mockReturnValue('/books/9/read')
    expect(renderHook(() => useTabBarVisible()).result.current).toBe(false)
  })
})
