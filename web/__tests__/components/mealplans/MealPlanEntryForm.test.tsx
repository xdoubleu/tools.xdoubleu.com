import { MemoryRouter } from 'react-router'
import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const { queueItemCategory } = vi.hoisted(() => ({
  queueItemCategory: vi.fn().mockResolvedValue(undefined)
}))

vi.mock('@/hooks/useIngredientCatalog', () => ({
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-produce', name: 'Produce' }
      ]
    }
  }),
  useItemCategories: () => ({ data: { items: [{ name: 'milk', categoryId: 'cat-dairy' }] } }),
  queueItemCategory
}))

import MealPlanEntryForm from '@/components/mealplans/MealPlanEntryForm'
import { create } from '@bufbuild/protobuf'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const onSave = vi.fn()
const onCancel = vi.fn()

const recipe = (id: string, name: string) => create(RecipeSchema, { id, name })
const suggestion = (id: string, name: string, servings = 1, fresh = false) => ({
  recipe: recipe(id, name),
  servings,
  fresh
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

beforeEach(() => vi.clearAllMocks())

describe('recipe tab save', () => {
  const renderRecipes = () =>
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[recipe('r1', 'Soep')]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )

  it('saves a typed exact recipe name on the first tap of Add', () => {
    renderRecipes()
    fireEvent.change(screen.getByRole('combobox', { name: 'Recipe' }), {
      target: { value: 'soep' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSave).toHaveBeenCalledWith('r1', '', 1, false)
  })

  it('shows an error when the typed name matches no recipe', () => {
    renderRecipes()
    fireEvent.change(screen.getByRole('combobox', { name: 'Recipe' }), {
      target: { value: 'Pizza' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByText(/No recipe with that name/)).toBeInTheDocument()
  })

  it('asks for a recipe when nothing is typed, and clears the error on typing', () => {
    renderRecipes()
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Pick a recipe.')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('combobox', { name: 'Recipe' }), {
      target: { value: 'S' }
    })
    expect(screen.queryByText('Pick a recipe.')).not.toBeInTheDocument()
  })

  it('restores the last valid servings on blur', () => {
    renderRecipes()
    const servings = screen.getByLabelText('Servings')
    fireEvent.change(servings, { target: { value: '3' } })
    fireEvent.change(servings, { target: { value: '' } })
    fireEvent.blur(servings)
    expect(servings).toHaveValue(3)
  })

  it('lists recipe options in the flow, not floating', () => {
    renderRecipes()
    fireEvent.change(screen.getByRole('combobox', { name: 'Recipe' }), {
      target: { value: 'oe' }
    })
    expect(screen.getByRole('listbox')).not.toHaveClass('absolute')
  })

  it('keeps the last valid servings when the field is cleared', () => {
    renderRecipes()
    const servings = screen.getByLabelText('Servings')
    fireEvent.change(servings, { target: { value: '4' } })
    fireEvent.change(servings, { target: { value: '' } })
    fireEvent.change(screen.getByRole('combobox', { name: 'Recipe' }), {
      target: { value: 'Soep' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSave).toHaveBeenCalledWith('r1', '', 4, false)
  })
})

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

    await waitFor(() => expect(queueItemCategory).toHaveBeenCalledWith('apple', 'cat-produce'))
    expect(onSave).toHaveBeenCalledWith('', 'apple', 1, false)
  })

  it('does not write when the row category already matches the catalog', async () => {
    renderForm('milk')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('', 'milk', 1, false))
    expect(queueItemCategory).not.toHaveBeenCalled()
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
    expect(screen.getByLabelText('Servings')).toBeInTheDocument()
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
    const servingsInput = screen.getByLabelText('Servings')
    if (!(servingsInput instanceof HTMLInputElement)) throw new Error('expected input')
    expect(servingsInput.value).toBe('4')
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('r1', '', 4, false))
  })

  it('styles the selected suggestion differently from the others', () => {
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
    expect(screen.getByRole('button', { name: 'Pasta' })).toHaveClass('bg-surface')
    expect(screen.getByRole('button', { name: 'Curry' })).toHaveClass('bg-surface')
    fireEvent.click(screen.getByRole('button', { name: 'Pasta' }))
    expect(screen.getByRole('button', { name: 'Pasta' })).toHaveClass('bg-accent')
    expect(screen.getByRole('button', { name: 'Curry' })).toHaveClass('bg-surface')
  })

  it('shows the New badge only on fresh suggestions', () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[recipe('r1', 'Pasta'), recipe('r2', 'Curry')]}
        suggestedRecipes={[suggestion('r1', 'Pasta'), suggestion('r2', 'Curry', 2, true)]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    const curry = screen.getByRole('button', { name: 'Curry (new)' })
    expect(curry.parentElement).toHaveTextContent('New')
    expect(screen.getByRole('button', { name: 'Pasta' }).parentElement).not.toHaveTextContent('New')
    expect(screen.getByRole('button', { name: 'Pasta' })).not.toHaveAttribute('aria-label')
    expect(screen.getByRole('button', { name: 'Curry (new)' })).toHaveAttribute(
      'aria-label',
      'Curry (new)'
    )
  })

  it('labels fresh suggestions as new', () => {
    render(
      <MealPlanEntryForm
        open
        title="Add"
        recipes={[recipe('r1', 'Pasta'), recipe('r2', 'Curry')]}
        suggestedRecipes={[suggestion('r1', 'Pasta'), suggestion('r2', 'Curry', 2, true)]}
        onSave={onSave}
        onCancel={onCancel}
      />
    )
    expect(screen.getByRole('button', { name: 'Curry (new)' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pasta' })).toBeInTheDocument()
  })

  it('warns when the selected recipe is a draft', () => {
    const draft = create(RecipeSchema, { id: 'd1', name: 'Soup', isDraft: true })
    render(
      <MemoryRouter>
        <MealPlanEntryForm
          open
          title="Add"
          recipes={[draft, recipe('r1', 'Pasta')]}
          suggestedRecipes={[suggestion('d1', 'Soup'), suggestion('r1', 'Pasta')]}
          onSave={onSave}
          onCancel={onCancel}
        />
      </MemoryRouter>
    )
    expect(screen.queryByText(/is a draft/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Soup' }))
    expect(screen.getByText(/Soup is a draft/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open recipe' })).toHaveAttribute('href', '/recipes/d1')
    fireEvent.click(screen.getByRole('button', { name: 'Pasta' }))
    expect(screen.queryByText(/is a draft/)).not.toBeInTheDocument()
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
    fireEvent.click(screen.getByRole('tab', { name: 'Custom' }))
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
