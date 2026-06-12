import React from 'react'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

const useContacts = jest.fn()
jest.mock('@/hooks/useContacts', () => ({
  useContacts: () => useContacts()
}))

import ShareModal from '@/components/recipes/ShareModal'

const contacts = [
  { id: 'c1', contactUserId: 'u-alice', displayName: 'Alice' },
  { id: 'c2', contactUserId: 'u-bob', displayName: 'Bob' }
]

beforeEach(() => {
  jest.clearAllMocks()
  useContacts.mockReturnValue({ data: { contacts } })
})

describe('ShareModal', () => {
  it('lists current shares with their permission and unshares', async () => {
    const onUnshare = jest.fn().mockResolvedValue(undefined)
    render(
      <ShareModal
        shares={[{ userId: 'u-bob', displayName: 'Bob', canEdit: false }]}
        onShare={jest.fn()}
        onUnshare={onUnshare}
        onClose={jest.fn()}
      />
    )

    const bobRow = screen.getByText('Bob').closest('li')
    if (!bobRow) throw new Error('expected Bob row')
    expect(within(bobRow).getByText('View only')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Unshare' }))
    expect(onUnshare).toHaveBeenCalledWith('u-bob')
  })

  it('shares with a selected contact and chosen permission', async () => {
    const onShare = jest.fn().mockResolvedValue(undefined)
    render(<ShareModal shares={[]} onShare={onShare} onUnshare={jest.fn()} onClose={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Contact to share with'), {
      target: { value: 'u-alice' }
    })
    fireEvent.change(screen.getByLabelText('Permission'), { target: { value: 'view' } })
    fireEvent.click(screen.getByRole('button', { name: 'Share' }))

    await waitFor(() => expect(onShare).toHaveBeenCalledWith('u-alice', false))
  })

  it('excludes already-shared contacts from the picker', () => {
    render(
      <ShareModal
        shares={[{ userId: 'u-alice', displayName: 'Alice', canEdit: true }]}
        onShare={jest.fn()}
        onUnshare={jest.fn()}
        onClose={jest.fn()}
      />
    )

    const select = screen.getByLabelText('Contact to share with') as HTMLSelectElement
    const optionValues = Array.from(select.options).map((o) => o.value)
    expect(optionValues).toContain('u-bob')
    expect(optionValues).not.toContain('u-alice')
  })

  it('prompts to add contacts when none exist', () => {
    useContacts.mockReturnValue({ data: { contacts: [] } })
    render(<ShareModal shares={[]} onShare={jest.fn()} onUnshare={jest.fn()} onClose={jest.fn()} />)
    expect(screen.getByText('Add contacts first to share with them.')).toBeInTheDocument()
  })
})
