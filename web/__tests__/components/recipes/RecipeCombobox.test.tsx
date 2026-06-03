import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent } from '@testing-library/react'
import RecipeCombobox from '@/components/recipes/RecipeCombobox'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const recipes = [
  create(RecipeSchema, { id: 'r1', name: 'Pasta Bolognese' }),
  create(RecipeSchema, { id: 'r2', name: 'Pasta Carbonara' }),
  create(RecipeSchema, { id: 'r3', name: 'Caesar Salad' })
]

describe('RecipeCombobox', () => {
  it('renders the text input', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    expect(screen.getByPlaceholderText(/recipe name or custom meal/i)).toBeInTheDocument()
  })

  it('shows filtered recipe list when typing', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'pasta' } })
    expect(screen.getByText('Pasta Bolognese')).toBeInTheDocument()
    expect(screen.getByText('Pasta Carbonara')).toBeInTheDocument()
    expect(screen.queryByText('Caesar Salad')).not.toBeInTheDocument()
  })

  it('calls onSelect with recipeId when recipe is clicked', () => {
    const onSelect = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={onSelect} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'pasta' } })
    fireEvent.mouseDown(screen.getByText('Pasta Bolognese'))
    expect(onSelect).toHaveBeenCalledWith('r1', '')
  })

  it('calls onSelect with customName when typing free text', () => {
    const onSelect = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={onSelect} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Eggs on toast' } })
    expect(onSelect).toHaveBeenLastCalledWith('', 'Eggs on toast')
  })

  it('navigates dropdown with arrow keys and selects with Enter', () => {
    const onSelect = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={onSelect} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'pasta' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelect).toHaveBeenLastCalledWith('r1', '')
  })

  it('closes dropdown on Escape', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'pasta' } })
    expect(screen.getByText('Pasta Bolognese')).toBeInTheDocument()
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByText('Pasta Bolognese')).not.toBeInTheDocument()
  })

  it('calls onEnter when Enter pressed with no dropdown item highlighted', () => {
    const onEnter = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} onEnter={onEnter} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'Soup' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onEnter).toHaveBeenCalled()
  })

  it('sets input value to recipe name after selection', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    const inputEl = screen.getByRole('textbox')
    if (!(inputEl instanceof HTMLInputElement)) throw new Error('expected HTMLInputElement')
    fireEvent.change(inputEl, { target: { value: 'pasta' } })
    fireEvent.mouseDown(screen.getByText('Pasta Carbonara'))
    expect(inputEl.value).toBe('Pasta Carbonara')
  })

  it('matches case-insensitively', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'CAESAR' } })
    expect(screen.getByText('Caesar Salad')).toBeInTheDocument()
  })

  it('Tab completes with the first filtered result when no item is highlighted', () => {
    const onSelect = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={onSelect} />)
    const input = screen.getByRole('textbox') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'pasta' } })
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(input.value).toBe('Pasta Bolognese')
    expect(onSelect).toHaveBeenLastCalledWith('r1', '')
  })

  it('Tab completes with the highlighted item when one is active', () => {
    const onSelect = jest.fn()
    render(<RecipeCombobox recipes={recipes} onSelect={onSelect} />)
    const input = screen.getByRole('textbox') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'pasta' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(input.value).toBe('Pasta Carbonara')
    expect(onSelect).toHaveBeenLastCalledWith('r2', '')
  })

  it('Tab does not prevent default when dropdown is closed', () => {
    render(<RecipeCombobox recipes={recipes} onSelect={jest.fn()} />)
    const input = screen.getByRole('textbox')
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true })
    const preventDefaultSpy = jest.spyOn(event, 'preventDefault')
    input.dispatchEvent(event)
    expect(preventDefaultSpy).not.toHaveBeenCalled()
  })
})
