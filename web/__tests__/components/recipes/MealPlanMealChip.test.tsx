import { render, screen, fireEvent } from '@testing-library/react'
import MealPlanMealChip from '@/components/recipes/MealPlanMealChip'
import { create } from '@bufbuild/protobuf'
import { PlanMealSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const recipe = create(RecipeSchema, { id: 'r1', name: 'Spaghetti bolognese' })

function makeMeal(overrides: Partial<Record<string, unknown>> = {}) {
  return create(PlanMealSchema, {
    id: 'm1',
    mealDate: '2026-05-25',
    mealSlot: 'breakfast',
    recipeId: 'r1',
    customName: '',
    servings: 1,
    ...overrides
  })
}

function renderChip(props: Partial<React.ComponentProps<typeof MealPlanMealChip>> = {}) {
  const handlers = {
    onMealClick: jest.fn(),
    onSwapClick: jest.fn(),
    onEditClick: jest.fn(),
    onDeleteMeal: jest.fn()
  }
  render(
    <MealPlanMealChip
      meal={makeMeal()}
      recipe={recipe}
      isSwapping={false}
      inSwapMode={false}
      {...handlers}
      {...props}
    />
  )
  return handlers
}

describe('MealPlanMealChip', () => {
  it('clamps the name by default and expands on body tap', () => {
    renderChip()
    const name = screen.getByText('Spaghetti bolognese')
    expect(name).toHaveClass('line-clamp-2')
    // Tap the chip body (the name's clickable ancestor) to expand
    fireEvent.click(name)
    expect(name).not.toHaveClass('line-clamp-2')
  })

  it('renders custom items as a bullet list', () => {
    renderChip({ meal: makeMeal({ recipeId: '', customName: 'Chicken\nRice' }) })
    expect(screen.getByText(/Chicken/)).toBeInTheDocument()
    expect(screen.getByText(/Rice/)).toBeInTheDocument()
  })

  it('opens the actions menu with Swap, Edit and Delete', () => {
    renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    expect(screen.getByRole('menuitem', { name: /Swap/i })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /Edit/i })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /Delete/i })).toBeInTheDocument()
  })

  it('Swap action calls onSwapClick', () => {
    const { onSwapClick } = renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Swap/i }))
    expect(onSwapClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'm1' }))
  })

  it('Edit action calls onEditClick', () => {
    const { onEditClick } = renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Edit/i }))
    expect(onEditClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'm1' }))
  })

  it('Delete action calls onDeleteMeal with the meal id', () => {
    const { onDeleteMeal } = renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    fireEvent.click(screen.getByRole('menuitem', { name: /Delete/i }))
    expect(onDeleteMeal).toHaveBeenCalledWith('m1')
  })

  it('closes the menu on Escape', () => {
    renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('closes the menu when clicking outside', () => {
    renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.mouseDown(document.body)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('opens the menu downward when there is room below', () => {
    renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    const menu = screen.getByRole('menu')
    expect(menu).toHaveClass('top-full')
    expect(menu).not.toHaveClass('bottom-full')
  })

  it('flips the menu upward when the trigger is near the bottom of the viewport', () => {
    const rect: DOMRect = {
      top: 700,
      bottom: 730,
      height: 30,
      left: 0,
      right: 0,
      width: 0,
      x: 0,
      y: 700,
      toJSON: () => ({})
    }
    const rectSpy = jest.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(rect)
    const originalInnerHeight = window.innerHeight
    Object.defineProperty(window, 'innerHeight', { value: 768, configurable: true })

    renderChip()
    fireEvent.click(screen.getByRole('button', { name: /Meal actions/i }))
    const menu = screen.getByRole('menu')
    expect(menu).toHaveClass('bottom-full')
    expect(menu).not.toHaveClass('top-full')

    rectSpy.mockRestore()
    Object.defineProperty(window, 'innerHeight', {
      value: originalInnerHeight,
      configurable: true
    })
  })

  it('hides the actions trigger in swap mode and routes body taps to onMealClick', () => {
    const { onMealClick } = renderChip({ inSwapMode: true })
    expect(screen.queryByRole('button', { name: /Meal actions/i })).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Spaghetti bolognese'))
    expect(onMealClick).toHaveBeenCalledWith(expect.objectContaining({ id: 'm1' }))
  })
})
