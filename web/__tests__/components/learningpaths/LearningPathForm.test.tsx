import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import {
  LearningPathSchema,
  ModuleSchema,
  ItemSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

const mockCreateLearningPath = jest.fn()
const mockUpdateLearningPath = jest.fn()

jest.mock('@/hooks/useLearningPaths', () => ({
  useCreateLearningPath: () => mockCreateLearningPath,
  useUpdateLearningPath: () => mockUpdateLearningPath
}))

beforeEach(() => jest.clearAllMocks())

describe('LearningPathForm (new)', () => {
  it('renders empty form fields', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByText('Title')).toBeInTheDocument()
    expect(screen.getByText('Goal')).toBeInTheDocument()
    expect(screen.getByText('Recurring routine')).toBeInTheDocument()
    expect(screen.getByText('Modules')).toBeInTheDocument()
    expect(screen.getByText('Resources')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Learning Path' })).toBeInTheDocument()
  })

  it('calls onCancel when Cancel is clicked', () => {
    const onCancel = jest.fn()
    render(<LearningPathForm onSave={jest.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('adds and removes a module', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add Module' }))
    expect(screen.getAllByPlaceholderText('Module title (e.g. Month 1)')).toHaveLength(2)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove module' })[0])
    expect(screen.getAllByPlaceholderText('Module title (e.g. Month 1)')).toHaveLength(1)
  })

  it('adds and removes an item within a module', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add Item' }))
    expect(screen.getAllByPlaceholderText('Description')).toHaveLength(2)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove item' })[0])
    expect(screen.getAllByPlaceholderText('Description')).toHaveLength(1)
  })

  it('adds and removes a resource', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add Resource' }))
    expect(screen.getAllByPlaceholderText('e.g. Book: The Go Programming Language')).toHaveLength(2)
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove resource' })[0])
    expect(screen.getAllByPlaceholderText('e.g. Book: The Go Programming Language')).toHaveLength(1)
  })

  it('calls createLearningPath and onSave on submit, dropping empty rows', async () => {
    const onSave = jest.fn()
    mockCreateLearningPath.mockResolvedValue({ learningPath: { id: 'new-id' } })
    render(<LearningPathForm onSave={onSave} onCancel={jest.fn()} />)

    const titleInput = screen.getAllByRole('textbox')[0]
    fireEvent.change(titleInput, { target: { value: 'Learn Go' } })
    fireEvent.change(screen.getByPlaceholderText('Module title (e.g. Month 1)'), {
      target: { value: 'Month 1' }
    })
    fireEvent.change(screen.getByPlaceholderText('Description'), {
      target: { value: 'Read the tour of Go' }
    })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Learn Go',
          modules: [
            expect.objectContaining({
              title: 'Month 1',
              items: [expect.objectContaining({ description: 'Read the tour of Go' })]
            })
          ],
          resources: []
        })
      )
      expect(onSave).toHaveBeenCalledWith('new-id')
    })
  })
})

describe('LearningPathForm (edit)', () => {
  const learningPath = create(LearningPathSchema, {
    id: 'lp-1',
    title: 'Existing Path',
    goal: 'Existing goal',
    routine: 'Daily',
    modules: [
      create(ModuleSchema, {
        title: 'M1',
        items: [create(ItemSchema, { type: 'read', description: 'item 1', completed: true })]
      })
    ],
    resources: []
  })

  it('pre-fills fields from the given learning path', () => {
    render(<LearningPathForm learningPath={learningPath} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByDisplayValue('Existing Path')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Existing goal')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Daily')).toBeInTheDocument()
    expect(screen.getByDisplayValue('M1')).toBeInTheDocument()
    expect(screen.getByDisplayValue('item 1')).toBeInTheDocument()
  })

  it('calls updateLearningPath preserving the completed flag on submit', async () => {
    const onSave = jest.fn()
    mockUpdateLearningPath.mockResolvedValue({ learningPath: { id: 'lp-1' } })
    render(<LearningPathForm learningPath={learningPath} onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() => {
      expect(mockUpdateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'lp-1',
          modules: [
            expect.objectContaining({
              items: [expect.objectContaining({ description: 'item 1', completed: true })]
            })
          ]
        })
      )
      expect(onSave).toHaveBeenCalledWith('lp-1')
    })
  })
})
