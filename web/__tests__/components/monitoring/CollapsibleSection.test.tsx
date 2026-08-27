import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import CollapsibleSection from '@/components/monitoring/CollapsibleSection'

describe('CollapsibleSection', () => {
  it('is collapsed by default and hides its children', () => {
    render(
      <CollapsibleSection title="Section title">
        <p>Section content</p>
      </CollapsibleSection>
    )

    expect(screen.getByRole('button', { name: 'Section title' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
    expect(screen.queryByText('Section content')).not.toBeInTheDocument()
  })

  it('toggles expanded/collapsed on click', () => {
    render(
      <CollapsibleSection title="Section title">
        <p>Section content</p>
      </CollapsibleSection>
    )

    const toggle = screen.getByRole('button', { name: 'Section title' })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Section content')).toBeInTheDocument()

    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Section content')).not.toBeInTheDocument()
  })

  it('respects defaultCollapsed={false}', () => {
    render(
      <CollapsibleSection title="Section title" defaultCollapsed={false}>
        <p>Section content</p>
      </CollapsibleSection>
    )

    expect(screen.getByRole('button', { name: 'Section title' })).toHaveAttribute(
      'aria-expanded',
      'true'
    )
    expect(screen.getByText('Section content')).toBeInTheDocument()
  })
})
