import { render } from '@testing-library/react'

jest.mock('@/app/recipes/[id]/RecipeClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="recipe-client">{id}</div>
}))

import RecipePage from '@/app/recipes/[id]/page'

describe('RecipePage', () => {
  it('renders without throwing', async () => {
    const params = Promise.resolve({ id: 'recipe-123' })
    const { getByTestId } = render(await RecipePage({ params }))
    expect(getByTestId('recipe-client')).toBeInTheDocument()
  })

  it('passes the id from params to RecipeClient', async () => {
    const params = Promise.resolve({ id: 'my-recipe-id' })
    const { getByText } = render(await RecipePage({ params }))
    expect(getByText('my-recipe-id')).toBeInTheDocument()
  })
})
