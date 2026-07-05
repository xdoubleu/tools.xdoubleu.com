import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() })
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/RecipeForm', () => ({
  __esModule: true,
  default: () => <div data-testid="recipe-form" />
}))

import NewRecipePage from '@/app/recipes/new/page'

describe('NewRecipePage', () => {
  it('renders the heading and form', () => {
    render(<NewRecipePage />)
    expect(screen.getByRole('heading', { name: 'New Recipe' })).toBeInTheDocument()
    expect(screen.getByTestId('recipe-form')).toBeInTheDocument()
  })
})
