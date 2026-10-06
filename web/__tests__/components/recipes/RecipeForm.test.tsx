import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RecipeForm from '@/components/recipes/RecipeForm'
import { RecipeSchema, IngredientSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockCreateRecipe = jest.fn()
const mockUpdateRecipe = jest.fn()

const mockShopping = {
  itemNames: [] as { name: string; categoryId: string; excluded: boolean }[],
  categories: [] as { id: string; name: string }[]
}
const mockQueueItemCategory = jest.fn().mockResolvedValue(undefined)
const mockQueueCategory = jest.fn()

jest.mock('@/hooks/useRecipes', () => ({
  useCreateRecipe: () => mockCreateRecipe,
  useUpdateRecipe: () => mockUpdateRecipe
}))

jest.mock('@/hooks/useShoppingList', () => ({
  useItemNames: () => ({ data: { names: mockShopping.itemNames } }),
  useCategories: () => ({ data: { categories: mockShopping.categories } }),
  queueCategory: (...args: unknown[]) => mockQueueCategory(...args),
  queueItemCategory: (...args: unknown[]) => mockQueueItemCategory(...args)
}))

jest.mock('@/lib/recipes/parseFraction', () => ({
  parseFraction: (s: string) => parseFloat(s) || 0
}))

beforeEach(() => {
  jest.clearAllMocks()
  mockShopping.itemNames = []
  mockShopping.categories = [
    { id: 'cat-produce', name: 'Produce' },
    { id: 'cat-dairy', name: 'Dairy' }
  ]
})

describe('RecipeForm (new recipe)', () => {
  it('renders form fields for a new recipe', () => {
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByText('Recipe Name')).toBeInTheDocument()
    expect(screen.getByText('Servings')).toBeInTheDocument()
    expect(screen.getByText('Ingredients')).toBeInTheDocument()
    expect(screen.getByText('Steps')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Recipe' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
  })

  it('gives the amount and category controls phone-sized tap targets', () => {
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)
    const amount = screen.getByPlaceholderText('e.g. 1/3')
    expect(amount).toHaveClass('min-w-0', 'flex-1', 'sm:w-16', 'sm:flex-none')
    expect(amount).not.toHaveClass('w-16')
    const category = screen.getByRole('combobox', { name: 'Category' })
    expect(category).toHaveClass('w-full', 'sm:w-auto')
  })

  it('calls onCancel when Cancel is clicked', () => {
    const onCancel = jest.fn()
    render(<RecipeForm onSave={jest.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('calls createRecipe and onSave on submit', async () => {
    const onSave = jest.fn()
    mockCreateRecipe.mockResolvedValue({ id: 'new-id', synced: true })
    render(<RecipeForm onSave={onSave} onCancel={jest.fn()} />)

    const nameInputs = screen.getAllByRole('textbox')
    fireEvent.change(nameInputs[0], { target: { value: 'Pasta' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalled()
      expect(onSave).toHaveBeenCalledWith('new-id', true)
    })
  })

  it('stays on the form when the server rejects the recipe', async () => {
    const onSave = jest.fn()
    const error = jest.spyOn(console, 'error').mockImplementation(() => {})
    mockCreateRecipe.mockRejectedValue(new Error('The server rejected the change'))
    render(<RecipeForm onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Pasta' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => expect(error).toHaveBeenCalled())
    expect(onSave).not.toHaveBeenCalled()
    error.mockRestore()
  })

  it('sends batchServings when set', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'new-id', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('e.g. 10'), { target: { value: '10' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(expect.objectContaining({ batchServings: 10 }))
    })
  })

  it('omits batchServings when field is empty', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'new-id', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(
        expect.objectContaining({ batchServings: undefined })
      )
    })
  })

  it('sends isDraft when the Draft box is checked', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'new-id', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByLabelText('Draft'))
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(expect.objectContaining({ isDraft: true }))
    })
  })

  it('defaults isDraft to false for a new recipe', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'new-id', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(expect.objectContaining({ isDraft: false }))
    })
  })

  it('adds a new ingredient row when Add Ingredient clicked', () => {
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)
    const initialRows = screen.getAllByPlaceholderText('Name')
    fireEvent.click(screen.getByRole('button', { name: 'Add Ingredient' }))
    const afterRows = screen.getAllByPlaceholderText('Name')
    expect(afterRows.length).toBe(initialRows.length + 1)
  })

  it('removes an ingredient row when Remove clicked', () => {
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)
    // A second row makes Remove appear.
    fireEvent.click(screen.getByRole('button', { name: 'Add Ingredient' }))
    const removeButtons = screen.getAllByRole('button', { name: 'Remove' })
    expect(removeButtons.length).toBe(2)
    fireEvent.click(removeButtons[0])
    expect(screen.getAllByPlaceholderText('Name')).toHaveLength(1)
  })

  it('suggests known ingredient names and auto-fills the category on selection', () => {
    mockShopping.itemNames = [
      { name: 'tomato', categoryId: 'cat-produce', excluded: false },
      { name: 'milk', categoryId: 'cat-dairy', excluded: false }
    ]
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'tom' } })
    expect(screen.getByText('tomato')).toBeInTheDocument()
    expect(screen.queryByText('milk')).not.toBeInTheDocument()

    fireEvent.mouseDown(screen.getByText('tomato'))

    const nameInput = screen.getByLabelText('Name') as HTMLInputElement
    expect(nameInput.value).toBe('tomato')
    expect((screen.getByLabelText('Category') as HTMLSelectElement).value).toBe('cat-produce')
  })

  it('assigns a selected existing category to a new ingredient on save', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'r-9', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'carrot' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'cat-produce' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => expect(mockQueueItemCategory).toHaveBeenCalledWith('carrot', 'cat-produce'))
  })

  it('creates a new category inline and assigns it on save', async () => {
    mockCreateRecipe.mockResolvedValue({ id: 'r-9', synced: true })
    mockQueueCategory.mockResolvedValue('cat-bakery')
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'eggs' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByLabelText('New category name'), { target: { value: 'Bakery' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockQueueCategory).toHaveBeenCalledWith('Bakery', mockShopping.categories)
      expect(mockQueueItemCategory).toHaveBeenCalledWith('eggs', 'cat-bakery')
    })
  })

  it('does not re-write a category that already matches the catalog', async () => {
    mockShopping.itemNames = [{ name: 'tomato', categoryId: 'cat-produce', excluded: false }]
    mockCreateRecipe.mockResolvedValue({ id: 'r-9', synced: true })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'tom' } })
    fireEvent.mouseDown(screen.getByText('tomato'))
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => expect(mockCreateRecipe).toHaveBeenCalled())
    expect(mockQueueItemCategory).not.toHaveBeenCalled()
  })
})

describe('RecipeForm (edit recipe)', () => {
  const existingRecipe = create(RecipeSchema, {
    id: 'r-1',
    name: 'Spaghetti',
    instructions: 'Boil water\nCook pasta',
    baseServings: 4,
    batchServings: 8,
    isDraft: true,
    ingredients: [create(IngredientSchema, { name: 'pasta', amount: 200, unit: 'g' })]
  })

  it('pre-fills fields from existing recipe including batchServings', () => {
    render(<RecipeForm recipe={existingRecipe} onSave={jest.fn()} onCancel={jest.fn()} />)
    const nameInputEl = screen.getAllByRole('textbox')[0]
    if (!(nameInputEl instanceof HTMLInputElement)) throw new Error('expected HTMLInputElement')
    expect(nameInputEl.value).toBe('Spaghetti')
    expect(screen.getByDisplayValue('pasta')).toBeInTheDocument()
    expect(screen.getByDisplayValue('8')).toBeInTheDocument()
    expect(screen.getByLabelText('Draft')).toBeChecked()
  })

  it('sends updated isDraft on submit', async () => {
    mockUpdateRecipe.mockResolvedValue({})
    render(<RecipeForm recipe={existingRecipe} onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByLabelText('Draft'))
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockUpdateRecipe).toHaveBeenCalledWith(expect.objectContaining({ isDraft: false }))
    })
  })

  it('pre-fills the ingredient category from the catalog', async () => {
    mockShopping.itemNames = [{ name: 'pasta', categoryId: 'cat-dairy', excluded: false }]
    render(<RecipeForm recipe={existingRecipe} onSave={jest.fn()} onCancel={jest.fn()} />)
    await waitFor(() =>
      expect((screen.getByLabelText('Category') as HTMLSelectElement).value).toBe('cat-dairy')
    )
  })

  it('sends updated batchServings on submit', async () => {
    const onSave = jest.fn()
    mockUpdateRecipe.mockResolvedValue({})
    render(<RecipeForm recipe={existingRecipe} onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('e.g. 10'), { target: { value: '12' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockUpdateRecipe).toHaveBeenCalledWith(expect.objectContaining({ batchServings: 12 }))
    })
  })

  it('clears batchServings when field is emptied', async () => {
    const onSave = jest.fn()
    mockUpdateRecipe.mockResolvedValue({})
    render(<RecipeForm recipe={existingRecipe} onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('e.g. 10'), { target: { value: '' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockUpdateRecipe).toHaveBeenCalledWith(
        expect.objectContaining({ batchServings: undefined })
      )
    })
  })

  it('calls updateRecipe and onSave on submit', async () => {
    const onSave = jest.fn()
    mockUpdateRecipe.mockResolvedValue({})
    render(<RecipeForm recipe={existingRecipe} onSave={onSave} onCancel={jest.fn()} />)
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockUpdateRecipe).toHaveBeenCalled()
      expect(onSave).toHaveBeenCalledWith('r-1', true)
    })
  })
})
