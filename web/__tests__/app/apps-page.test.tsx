import { create } from '@bufbuild/protobuf'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import AppsPage from '@/app/apps/page'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

vi.mock('@/hooks/useAuth', () => ({ useCurrentUser: vi.fn() }))

import { useCurrentUser } from '@/hooks/useAuth'

function user(role: string, appAccess: string[]) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(useCurrentUser).mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role, appAccess })
  })
}

const renderPage = () =>
  render(
    <MemoryRouter>
      <AppsPage />
    </MemoryRouter>
  )

describe('AppsPage', () => {
  it('shows a loading state until the user is known', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined })
    renderPage()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('lists the apps the user can open, grouped', () => {
    user('user', ['feeds', 'trains'])
    renderPage()
    expect(screen.getByRole('heading', { level: 1, name: 'Apps' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Feeds/ })).toHaveAttribute('href', '/feeds')
    expect(screen.getByRole('link', { name: /Trains/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Books/ })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Transit' })).toBeInTheDocument()
  })

  it('filters by name or description and drops empty groups', () => {
    user('user', ['feeds', 'trains'])
    renderPage()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search apps' }), {
      target: { value: '  TRAIN ' }
    })
    expect(screen.getByRole('link', { name: /Trains/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Feeds/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Learning' })).not.toBeInTheDocument()
  })

  it('matches descriptions case-insensitively', () => {
    user('admin', [])
    renderPage()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search apps' }), {
      target: { value: 'steam' }
    })
    expect(screen.getByRole('link', { name: /Games/ })).toBeInTheDocument()
  })

  it('says so when nothing matches', () => {
    user('user', ['feeds'])
    renderPage()
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search apps' }), {
      target: { value: 'zzz' }
    })
    expect(screen.getByText('No apps match your search.')).toBeInTheDocument()
  })
})
