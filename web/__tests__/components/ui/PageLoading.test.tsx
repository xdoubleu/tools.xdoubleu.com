import React from 'react'
import { render, screen } from '@testing-library/react'
import { PageLoading } from '@/components/ui/PageLoading'

describe('PageLoading', () => {
  it('renders the standard loading copy', () => {
    render(<PageLoading />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})
