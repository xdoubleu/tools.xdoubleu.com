import React from 'react'
import { render, screen } from '@testing-library/react'
import LoadingComponent from '@/app/learningpaths/list/loading'

describe('learningpaths list loading', () => {
  it('renders the shared loading state', () => {
    render(<LoadingComponent />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
