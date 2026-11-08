import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import MealPlanGroupFilter from '@/components/shoppinglist/MealPlanGroupFilter'

const groups = [
  { recipeName: 'Pasta', groupName: 'Sauce' },
  { recipeName: 'Salad', groupName: 'Dressing' }
]

describe('MealPlanGroupFilter', () => {
  it('renders a toggle per group with the recipe name', () => {
    render(<MealPlanGroupFilter groups={groups} excludedGroups={new Set()} onToggle={vi.fn()} />)
    expect(screen.getByRole('toolbar', { name: 'Include ingredient groups' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sauce (Pasta)' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dressing (Salad)' })).toBeInTheDocument()
  })

  it('presses included groups and releases excluded ones', () => {
    render(
      <MealPlanGroupFilter groups={groups} excludedGroups={new Set(['Sauce'])} onToggle={vi.fn()} />
    )
    const [sauce, dressing] = screen.getAllByRole('button')
    expect(sauce).toHaveAttribute('aria-pressed', 'false')
    expect(dressing).toHaveAttribute('aria-pressed', 'true')
  })

  it('calls onToggle with the group name when clicked', () => {
    const onToggle = vi.fn()
    render(<MealPlanGroupFilter groups={groups} excludedGroups={new Set()} onToggle={onToggle} />)
    fireEvent.click(screen.getAllByRole('button')[0])
    expect(onToggle).toHaveBeenCalledWith('Sauce')
  })

  it('renders nothing when there are no groups', () => {
    const { container } = render(
      <MealPlanGroupFilter groups={[]} excludedGroups={new Set()} onToggle={vi.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
  })
})
