import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const shareBook = jest.fn().mockResolvedValue(undefined)
const mutateBook = jest.fn().mockResolvedValue(undefined)

jest.mock('@/hooks/useRecipes', () => ({
  useRecipeBookShares: () => ({
    data: { shares: [{ userId: 'u-bob', displayName: 'Bob', canEdit: true }] },
    mutate: mutateBook
  }),
  useShareRecipeBook: () => shareBook,
  useUnshareRecipeBook: () => jest.fn()
}))

jest.mock('@/hooks/useShoppingList', () => ({
  useShoppingListShares: () => ({ data: { shares: [] }, mutate: jest.fn() }),
  useShareShoppingList: () => jest.fn(),
  useUnshareShoppingList: () => jest.fn()
}))

jest.mock('@/hooks/useMealPlans', () => ({
  useSharePlan: () => jest.fn(),
  useUnsharePlan: () => jest.fn()
}))

jest.mock('@/hooks/useContacts', () => ({
  useContacts: () => ({
    data: { contacts: [{ id: 'c1', contactUserId: 'u-x', displayName: 'X' }] }
  })
}))

jest.mock('swr', () => ({
  __esModule: true,
  default: () => ({ data: [], mutate: jest.fn() })
}))

jest.mock('@/lib/client', () => ({ createServiceClient: () => ({}) }))
jest.mock('@/lib/gen/mealplans/v1/mealplans_pb', () => ({ MealPlansService: {} }))

import SharingPageClient from '@/components/sharing/SharingPageClient'

beforeEach(() => jest.clearAllMocks())

describe('SharingPage', () => {
  it('lists recipe book shares and opens the manage modal', async () => {
    render(<SharingPageClient />)

    expect(screen.getByText('Recipe book')).toBeInTheDocument()
    expect(screen.getByText('Shopping list')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()

    // Open the recipe book manage modal (first Manage button).
    fireEvent.click(screen.getAllByRole('button', { name: 'Manage' })[0])
    await waitFor(() => expect(screen.getByText('Share recipe book')).toBeInTheDocument())
  })
})
