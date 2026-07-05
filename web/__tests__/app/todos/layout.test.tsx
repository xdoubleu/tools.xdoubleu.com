import React from 'react'
import { render, screen } from '@testing-library/react'

import TodosLayout from '@/app/todos/layout'

describe('TodosLayout', () => {
  it('renders children inside the page container', () => {
    render(
      <TodosLayout>
        <div data-testid="child" />
      </TodosLayout>
    )
    expect(screen.getByTestId('child')).toBeInTheDocument()
  })
})
