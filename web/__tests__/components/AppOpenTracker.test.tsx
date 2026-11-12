import { create } from '@bufbuild/protobuf'
import { render } from '@testing-library/react'
import AppOpenTracker from '@/components/AppOpenTracker'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

const mockRecord = vi.fn()
vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn(),
  useRecordAppOpen: () => mockRecord
}))
vi.mock('@/hooks/navigation', () => ({ usePathname: vi.fn() }))

import { usePathname } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'

function signedIn(role = 'user', appAccess = ['recipes', 'trains']) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(useCurrentUser).mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role, appAccess })
  })
}

const at = (path: string) => vi.mocked(usePathname).mockReturnValue(path)

beforeEach(() => {
  vi.clearAllMocks()
  mockRecord.mockResolvedValue({})
})

describe('AppOpenTracker', () => {
  it('renders nothing', () => {
    signedIn()
    at('/')
    expect(render(<AppOpenTracker />).container.firstChild).toBeNull()
  })

  it('reports the app the user lands in', () => {
    signedIn()
    at('/recipes/list')
    render(<AppOpenTracker />)
    expect(mockRecord).toHaveBeenCalledTimes(1)
    expect(mockRecord).toHaveBeenCalledWith('recipes')
  })

  it('reports once per switch, not per page', () => {
    signedIn()
    at('/recipes/list')
    const { rerender } = render(<AppOpenTracker />)
    at('/recipes/abc')
    rerender(<AppOpenTracker />)
    expect(mockRecord).toHaveBeenCalledTimes(1)

    at('/trains')
    rerender(<AppOpenTracker />)
    expect(mockRecord).toHaveBeenLastCalledWith('trains')

    at('/recipes/list')
    rerender(<AppOpenTracker />)
    expect(mockRecord).toHaveBeenCalledTimes(3)
  })

  it('ignores the home page, account pages and unknown paths', () => {
    signedIn()
    at('/')
    const { rerender } = render(<AppOpenTracker />)
    for (const path of ['/', '/apps', '/settings', '/family', '/nowhere']) {
      at(path)
      rerender(<AppOpenTracker />)
    }
    expect(mockRecord).not.toHaveBeenCalled()
  })

  it('does not report leaving an app for the home page', () => {
    signedIn()
    at('/recipes/list')
    const { rerender } = render(<AppOpenTracker />)
    at('/')
    rerender(<AppOpenTracker />)
    expect(mockRecord).toHaveBeenCalledTimes(1)
  })

  it('ignores apps the user cannot open', () => {
    signedIn('user', ['recipes'])
    at('/trains')
    render(<AppOpenTracker />)
    expect(mockRecord).not.toHaveBeenCalled()
  })

  it('does nothing when signed out', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined })
    at('/recipes/list')
    render(<AppOpenTracker />)
    expect(mockRecord).not.toHaveBeenCalled()
  })

  it('swallows a failed report', async () => {
    signedIn()
    mockRecord.mockRejectedValueOnce(new Error('offline'))
    at('/recipes/list')
    render(<AppOpenTracker />)
    await Promise.resolve()
    expect(mockRecord).toHaveBeenCalledTimes(1)
  })
})
