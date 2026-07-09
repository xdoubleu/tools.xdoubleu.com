import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const setItemCategory = jest.fn().mockResolvedValue({})
const mutate = jest.fn().mockResolvedValue(undefined)

jest.mock('@/hooks/useShoppingList', () => ({
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-produce', name: 'Produce' }
      ]
    }
  }),
  useItemCategories: () => ({ data: { items: [{ name: 'milk', categoryId: 'cat-dairy' }] } })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ setItemCategory })
}))

jest.mock('swr', () => ({
  __esModule: true,
  useSWRConfig: () => ({ mutate })
}))

import MealPlanEntryForm from '@/components/recipes/MealPlanEntryForm'
import { create } from '@bufbuild/protobuf'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const onSave = jest.fn()
const onCancel = jest.fn()

const recipe = (id: string, name: string) => create(RecipeSchema, { id, name })
const suggestion = (id: string, name: string, servings = 1) => ({
  recipe: recipe(id, name),
  servings
})

function renderForm(initialCustomName: string) {
  return render(
    <MealPlanEntryForm
      open
      title="Add"
      recipes={[]}
      initialCustomName={initialCustomName}
      onSave={onSave}
      onCancel={onCancel}
    />
  )
}

beforeEach(() => jest.clearAllMocks())

describe('MealPlanEntryForm category picker', () => {
  it('pre-fills a row category from the name->category catalog', () => {
    renderForm('milk')
    const select = screen.getByLabelText('Category for item 1') as HTMLSelectElement
    expect(select.value).toBe('cat-dairy')
  })

  it('writes the chosen category to the catalog on save', async () => {
    renderForm('apple')
    fireEvent.change(screen.getByLabelText('Category for item 1'), {
      target: { value: 'cat-produce' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(setItemCategory).toHaveBeenCalledWith({ name: 'apple', categoryId: 'cat-produce' })
    )
    expect(mutate).toHaveBeenCalledWith('/shoppinglist/item-categories')
    expect(onSave).toHaveBeenCalledWith('', 'apple', 1, false)
  })

  it('does not write when the row category already matches the catalog', async () => {
    renderForm('milk')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('', 'milk', 1, false))
    expect(setItemCategory).not.toHaveBeenCalled()
  })

  it('encodes unit into customName on save', async () => {
    renderForm('apple')
    fireEvent.change(screen.getByLabelText('Amount for item 1'), { target: { value: '3' } })
    fireEvent.change(screen.getByLabelText('Unit for item 1'), { target: { value: 'kg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('', 'apple\t3\tkg', 1, false))
  })

  it('defaults to the recipe tab for a new entry', () => {
    render(<MealPlanEntryForm open title="Add" recipes={[]} onSave={onSave} onCancel={onCancel} />)
    expect(screen.getByPlaceholderText('Servings')).toBeInTheDocument()
    expect(screen.queryByText('+ Add item')).not.toBeInTheDocument()
  })

  it('renders suggestion chips on the recipe tab', () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[recipe('r1', 'Pasta'), recipe('r2', 'Curry')]}
        suggestedRecipes={[suggestion('r1', 'Pasta'), suggestion('r2', 'Curry')]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    expect(screen.getByText('Suggestions')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pasta' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Curry' })).toBeInTheDocument()
  })

  it('selects a recipe and pre-fills servings when a suggestion chip is clicked', async () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        saveLabel="Add"
        recipes={[recipe('r1', 'Pasta')]}
        suggestedRecipes={[suggestion('r1', 'Pasta', 4)]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Pasta' }))
    const servingsInput = screen.getByPlaceholderText('Servings')
    if (!(servingsInput instanceof HTMLInputElement)) throw new Error('expected input')
    expect(servingsInput.value).toBe('4')
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('r1', '', 4, false))
  })

  it('renders no suggestions when none are provided', () => {
    render(<MealPlanEntryForm open title="Add" recipes={[]} onSave={onSave} onCancel={onCancel} />)
    expect(screen.queryByText('Suggestions')).not.toBeInTheDocument()
  })

  it('hides suggestions on the custom tab', () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[recipe('r1', 'Pasta')]}
        suggestedRecipes={[suggestion('r1', 'Pasta')]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Custom' }))
    expect(screen.queryByText('Suggestions')).not.toBeInTheDocument()
  })

  it('hides the category selector when keep off shopping list is checked', async () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[]}
        initialCustomName="apple"
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    expect(screen.getByLabelText('Category for item 1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.queryByLabelText('Category for item 1')).not.toBeInTheDocument()
  })

  it('hides amount and unit when keep off shopping list is checked', async () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[]}
        initialCustomName="apple"
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    expect(screen.getByLabelText('Amount for item 1')).toBeInTheDocument()
    expect(screen.getByLabelText('Unit for item 1')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.queryByLabelText('Amount for item 1')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Unit for item 1')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.getByLabelText('Amount for item 1')).toBeInTheDocument()
    expect(screen.getByLabelText('Unit for item 1')).toBeInTheDocument()
  })
})
