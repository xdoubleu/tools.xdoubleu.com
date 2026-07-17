import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import MealPlanGroupFilter from '@/components/shoppinglist/MealPlanGroupFilter'

const groups = [
  { recipeName: 'Pasta', groupName: 'Sauce' },
  { recipeName: 'Salad', groupName: 'Dressing' }
]

describe('MealPlanGroupFilter', () => {
  it('renders a checkbox per group with the recipe name', () => {
    render(<MealPlanGroupFilter groups={groups} excludedGroups={new Set()} onToggle={jest.fn()} />)
    expect(screen.getByText('Sauce')).toBeInTheDocument()
    expect(screen.getByText('(Pasta)')).toBeInTheDocument()
    expect(screen.getByText('Dressing')).toBeInTheDocument()
    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
  })

  it('checks included groups and unchecks excluded ones', () => {
    render(
      <MealPlanGroupFilter
        groups={groups}
        excludedGroups={new Set(['Sauce'])}
        onToggle={jest.fn()}
      />
    )
    const [sauce, dressing] = screen.getAllByRole('checkbox')
    expect(sauce).not.toBeChecked()
    expect(dressing).toBeChecked()
  })

  it('calls onToggle with the group name when clicked', () => {
    const onToggle = jest.fn()
    render(<MealPlanGroupFilter groups={groups} excludedGroups={new Set()} onToggle={onToggle} />)
    fireEvent.click(screen.getAllByRole('checkbox')[0])
    expect(onToggle).toHaveBeenCalledWith('Sauce')
  })

  it('renders nothing when there are no groups', () => {
    const { container } = render(
      <MealPlanGroupFilter groups={[]} excludedGroups={new Set()} onToggle={jest.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
  })
})
