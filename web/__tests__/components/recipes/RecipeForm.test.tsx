import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RecipeForm from '@/components/recipes/RecipeForm'
import { RecipeSchema, IngredientSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockCreateRecipe = jest.fn()
const mockUpdateRecipe = jest.fn()

jest.mock('@/hooks/useRecipes', () => ({
  useCreateRecipe: () => mockCreateRecipe,
  useUpdateRecipe: () => mockUpdateRecipe
}))

jest.mock('@/lib/recipes/parseFraction', () => ({
  parseFraction: (s: string) => parseFloat(s) || 0
}))

describe('RecipeForm (new recipe)', () => {
  beforeEach(() => {
    mockCreateRecipe.mockReset()
    mockUpdateRecipe.mockReset()
  })

  it('renders form fields for a new recipe', () => {
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByText('Recipe Name')).toBeInTheDocument()
    expect(screen.getByText('Servings')).toBeInTheDocument()
    expect(screen.getByText('Ingredients')).toBeInTheDocument()
    expect(screen.getByText('Steps')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Recipe' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
  })

  it('calls onCancel when Cancel is clicked', () => {
    const onCancel = jest.fn()
    render(<RecipeForm onSave={jest.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('calls createRecipe and onSave on submit', async () => {
    const onSave = jest.fn()
    mockCreateRecipe.mockResolvedValue({ recipe: { id: 'new-id' } })
    render(<RecipeForm onSave={onSave} onCancel={jest.fn()} />)

    const nameInputs = screen.getAllByRole('textbox')
    fireEvent.change(nameInputs[0], { target: { value: 'Pasta' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalled()
      expect(onSave).toHaveBeenCalledWith('new-id')
    })
  })

  it('sends batchServings when set', async () => {
    mockCreateRecipe.mockResolvedValue({ recipe: { id: 'new-id' } })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('e.g. 10'), { target: { value: '10' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(expect.objectContaining({ batchServings: 10 }))
    })
  })

  it('omits batchServings when field is empty', async () => {
    mockCreateRecipe.mockResolvedValue({ recipe: { id: 'new-id' } })
    render(<RecipeForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.submit(screen.getByRole('button', { name: 'Save Recipe' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateRecipe).toHaveBeenCalledWith(
        expect.objectContaining({ batchServings: undefined })
      )
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
    // Add a second row first (so Remove button appears)
    fireEvent.click(screen.getByRole('button', { name: 'Add Ingredient' }))
    const removeButtons = screen.getAllByRole('button', { name: 'Remove' })
    expect(removeButtons.length).toBe(2)
    fireEvent.click(removeButtons[0])
    expect(screen.getAllByPlaceholderText('Name')).toHaveLength(1)
  })
})

describe('RecipeForm (edit recipe)', () => {
  const existingRecipe = create(RecipeSchema, {
    id: 'r-1',
    name: 'Spaghetti',
    instructions: 'Boil water\nCook pasta',
    baseServings: 4,
    batchServings: 8,
    ingredients: [create(IngredientSchema, { name: 'pasta', amount: 200, unit: 'g' })]
  })

  it('pre-fills fields from existing recipe including batchServings', () => {
    render(<RecipeForm recipe={existingRecipe} onSave={jest.fn()} onCancel={jest.fn()} />)
    const nameInputEl = screen.getAllByRole('textbox')[0]
    if (!(nameInputEl instanceof HTMLInputElement)) throw new Error('expected HTMLInputElement')
    expect(nameInputEl.value).toBe('Spaghetti')
    expect(screen.getByDisplayValue('pasta')).toBeInTheDocument()
    expect(screen.getByDisplayValue('8')).toBeInTheDocument()
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
      expect(onSave).toHaveBeenCalledWith('r-1')
    })
  })
})
