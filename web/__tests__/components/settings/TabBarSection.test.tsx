import { create } from '@bufbuild/protobuf'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { TabBarSection } from '@/components/settings/TabBarSection'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

const mockSetTabPins = vi.fn()
vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn(),
  useSetTabPins: () => mockSetTabPins
}))

import { useCurrentUser } from '@/hooks/useAuth'

function user(init: { appAccess: string[]; tabPins?: string[]; tabPinsCustomised?: boolean }) {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  vi.mocked(useCurrentUser).mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role: 'user', ...init })
  })
}

const pinned = () => within(screen.getByRole('list', { name: 'Pinned apps' }))
const others = () => within(screen.getByRole('list', { name: 'Other apps' }))

beforeEach(() => {
  vi.clearAllMocks()
  mockSetTabPins.mockResolvedValue(undefined)
})

describe('TabBarSection', () => {
  it('renders nothing until the user is known', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined })
    expect(render(<TabBarSection />).container.firstChild).toBeNull()
  })

  it('lists the current pins and the other apps', () => {
    user({ appAccess: ['books', 'feeds', 'mealplans', 'trains'] })
    render(<TabBarSection />)
    expect(
      pinned()
        .getAllByRole('listitem')
        .map((li) => li.textContent)
    ).toEqual([
      expect.stringContaining('Books'),
      expect.stringContaining('Feeds'),
      expect.stringContaining('Meal plans')
    ])
    expect(
      others()
        .getAllByRole('listitem')
        .map((li) => li.textContent)
    ).toEqual([expect.stringContaining('Trains')])
    expect(screen.getByText(/up to 3/)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText('Nothing pinned.')).not.toBeInTheDocument()
    expect(screen.queryByText('Settings')).not.toBeInTheDocument()
  })

  it('unpins an app, saving the rest in order', async () => {
    user({ appAccess: ['books', 'feeds', 'mealplans'] })
    render(<TabBarSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Unpin Feeds' }))
    await waitFor(() => expect(mockSetTabPins).toHaveBeenCalledWith(['books', 'mealplans']))
  })

  it('pins an app at the end', async () => {
    user({ appAccess: ['books', 'trains'], tabPins: ['books'], tabPinsCustomised: true })
    render(<TabBarSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Pin Trains' }))
    await waitFor(() => expect(mockSetTabPins).toHaveBeenCalledWith(['books', 'trains']))
  })

  it('stops offering pins once three are chosen', () => {
    user({ appAccess: ['books', 'feeds', 'mealplans', 'trains'] })
    render(<TabBarSection />)
    expect(screen.getByRole('button', { name: 'Pin Trains' })).toBeDisabled()
  })

  it('moves an app up or down', async () => {
    user({ appAccess: ['books', 'feeds', 'mealplans'] })
    render(<TabBarSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Move Feeds up' }))
    await waitFor(() =>
      expect(mockSetTabPins).toHaveBeenLastCalledWith(['feeds', 'books', 'mealplans'])
    )
    fireEvent.click(screen.getByRole('button', { name: 'Move Feeds down' }))
    await waitFor(() =>
      expect(mockSetTabPins).toHaveBeenLastCalledWith(['books', 'mealplans', 'feeds'])
    )
  })

  it('cannot move the first app up or the last down', () => {
    user({ appAccess: ['books', 'feeds'] })
    render(<TabBarSection />)
    expect(screen.getByRole('button', { name: 'Move Books up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Feeds down' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Books down' })).toBeEnabled()
  })

  it('has no other-apps list when everything is pinned', () => {
    user({ appAccess: ['books'] })
    render(<TabBarSection />)
    expect(screen.queryByRole('list', { name: 'Other apps' })).not.toBeInTheDocument()
  })

  it('says so when nothing is pinned', () => {
    user({ appAccess: ['books'], tabPins: [], tabPinsCustomised: true })
    render(<TabBarSection />)
    expect(screen.getByText('Nothing pinned.')).toBeInTheDocument()
  })

  it('locks the buttons while a save is in flight', async () => {
    user({ appAccess: ['books', 'feeds', 'mealplans', 'trains'] })
    let finish: () => void = () => {}
    mockSetTabPins.mockReturnValueOnce(new Promise<void>((resolve) => (finish = resolve)))
    render(<TabBarSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Unpin Books' }))
    expect(screen.getByRole('button', { name: 'Unpin Feeds' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Feeds up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move Books down' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Pin Trains' })).toBeDisabled()
    finish()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Unpin Feeds' })).toBeEnabled())
  })

  it('shows an error when saving fails, and clears it on the next try', async () => {
    user({ appAccess: ['books'] })
    mockSetTabPins.mockRejectedValueOnce(new Error('boom'))
    render(<TabBarSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Unpin Books' }))
    expect(await screen.findByText('Could not save the tab bar. Try again.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Unpin Books' }))
    await waitFor(() =>
      expect(screen.queryByText('Could not save the tab bar. Try again.')).not.toBeInTheDocument()
    )
  })
})
