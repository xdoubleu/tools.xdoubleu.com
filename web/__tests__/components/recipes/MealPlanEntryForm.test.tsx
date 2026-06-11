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

const onSave = jest.fn()
const onCancel = jest.fn()

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
})
