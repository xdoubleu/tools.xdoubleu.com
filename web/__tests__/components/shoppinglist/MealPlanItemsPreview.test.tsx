import React from 'react'
import { render, screen } from '@testing-library/react'
import MealPlanItemsPreview from '@/components/shoppinglist/MealPlanItemsPreview'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

describe('MealPlanItemsPreview', () => {
  it('combines items sharing a name and unit across recipes and lists origins', () => {
    const mealItems: ShoppingItem[] = [
      { name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta' },
      { name: 'garlic', amount: '3', unit: 'cloves', recipeName: 'Curry' }
    ]
    render(<MealPlanItemsPreview mealItems={mealItems} />)

    expect(screen.getByText('From meal plans')).toBeInTheDocument()
    // 2 + 3 cloves are summed into a single combined row.
    expect(screen.getByText(/5 cloves — garlic/)).toBeInTheDocument()
    // Both recipes are shown as origins.
    expect(screen.getByText(/Pasta: 2 cloves, Curry: 3 cloves/)).toBeInTheDocument()
  })

  it('shows a loading state', () => {
    render(<MealPlanItemsPreview mealItems={[]} isLoading />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders nothing when there are no meal-plan items', () => {
    const { container } = render(<MealPlanItemsPreview mealItems={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders rows read-only (no buttons)', () => {
    const mealItems: ShoppingItem[] = [
      { name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta' }
    ]
    render(<MealPlanItemsPreview mealItems={mealItems} />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
