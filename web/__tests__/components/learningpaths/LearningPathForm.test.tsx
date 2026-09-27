import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import {
  LearningPathSchema,
  ModuleSchema,
  ItemSchema,
  ResourceSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

const mockCreateLearningPath = jest.fn()
const mockUpdateLearningPath = jest.fn()

jest.mock('@/hooks/useLearningPaths', () => ({
  useCreateLearningPath: () => mockCreateLearningPath,
  useUpdateLearningPath: () => mockUpdateLearningPath
}))

// Thin stand-in so the form's link/unlink wiring can be tested without
// driving the picker's search UI (covered by its own tests).
jest.mock('@/components/learningpaths/ResourceLinkPicker', () => {
  const MockResourceLinkPicker = (props: {
    linkedBook?: { id: string; title: string }
    linkedFeedItem?: { id: string; title: string }
    onLinkBook: (v: { id: string; title: string }) => void
    onLinkFeedItem: (v: { id: string; title: string }) => void
    onUnlink: () => void
  }) => (
    <div>
      {props.linkedBook && <span>Linked book: {props.linkedBook.title}</span>}
      {props.linkedFeedItem && <span>Linked feed item: {props.linkedFeedItem.title}</span>}
      <button type="button" onClick={() => props.onLinkBook({ id: 'b1', title: 'Dune' })}>
        Mock link book
      </button>
      <button type="button" onClick={() => props.onLinkFeedItem({ id: 'f1', title: 'An Article' })}>
        Mock link feed item
      </button>
      <button type="button" onClick={props.onUnlink}>
        Mock unlink
      </button>
    </div>
  )
  return {
    __esModule: true,
    default: MockResourceLinkPicker,
    BookLinkSearch: ({ onPick }: { onPick: (v: { id: string; title: string }) => void }) => (
      <button type="button" onClick={() => onPick({ id: 'b1', title: 'Dune' })}>
        Mock pick item book
      </button>
    )
  }
})

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

  it('links a book to a resource and submits its id', async () => {
    const onSave = jest.fn()
    mockCreateLearningPath.mockResolvedValue({ learningPath: { id: 'new-id' } })
    render(<LearningPathForm onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mock link book' }))
    expect(screen.getByText('Linked book: Dune')).toBeInTheDocument()

    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({
          resources: [expect.objectContaining({ text: '', linkedBookId: 'b1' })]
        })
      )
      expect(onSave).toHaveBeenCalledWith('new-id')
    })
  })

  it('links a feed item, which clears a previously linked book on the same resource', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mock link book' }))
    expect(screen.getByText('Linked book: Dune')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Mock link feed item' }))
    expect(screen.getByText('Linked feed item: An Article')).toBeInTheDocument()
    expect(screen.queryByText('Linked book: Dune')).not.toBeInTheDocument()
  })

  it('unlinks a resource', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mock link book' }))
    expect(screen.getByText('Linked book: Dune')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Mock unlink' }))
    expect(screen.queryByText('Linked book: Dune')).not.toBeInTheDocument()
  })

  it('includes a resource with only a linked book (no text) in the submitted payload', async () => {
    mockCreateLearningPath.mockResolvedValue({ learningPath: { id: 'new-id' } })
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mock link book' }))
    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({
          resources: [{ text: '', linkedBookId: 'b1', linkedFeedItemId: undefined }]
        })
      )
    })
  })

  it('links a book to an item and submits its id', async () => {
    const onSave = jest.fn()
    mockCreateLearningPath.mockResolvedValue({ learningPath: { id: 'new-id' } })
    render(<LearningPathForm onSave={onSave} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('Module title (e.g. Month 1)'), {
      target: { value: 'M1' }
    })
    fireEvent.change(screen.getByPlaceholderText('Description'), {
      target: { value: 'Read Dune' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Link book' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mock pick item book' }))

    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() => {
      expect(mockCreateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({
          modules: [
            expect.objectContaining({
              items: [expect.objectContaining({ linkedBookId: 'b1' })]
            })
          ]
        })
      )
      expect(onSave).toHaveBeenCalledWith('new-id')
    })
  })

  it('unlinks a book from an item', () => {
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Link book' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mock pick item book' }))
    expect(screen.getByText('📚 Dune')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Unlink' }))
    expect(screen.queryByText('📚 Dune')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Link book' })).toBeInTheDocument()
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
    resources: [
      create(ResourceSchema, {
        text: 'A note',
        linkedBookId: 'b1',
        linkedBook: { title: 'Dune' }
      })
    ]
  })

  it('pre-fills fields from the given learning path', () => {
    render(<LearningPathForm learningPath={learningPath} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByDisplayValue('Existing Path')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Existing goal')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Daily')).toBeInTheDocument()
    expect(screen.getByDisplayValue('M1')).toBeInTheDocument()
    expect(screen.getByDisplayValue('item 1')).toBeInTheDocument()
    expect(screen.getByDisplayValue('A note')).toBeInTheDocument()
    expect(screen.getByText('Linked book: Dune')).toBeInTheDocument()
  })

  it('pre-fills a book-linked item in edit mode', () => {
    const withBookItem = create(LearningPathSchema, {
      id: 'lp-1',
      title: 'Reading Path',
      modules: [
        create(ModuleSchema, {
          title: 'M1',
          items: [
            create(ItemSchema, {
              type: 'read',
              description: 'Read Dune',
              linkedBookId: 'b9',
              linkedBook: { title: 'Dune', progressPercent: 100 }
            })
          ]
        })
      ]
    })
    render(<LearningPathForm learningPath={withBookItem} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByText('📚 Dune')).toBeInTheDocument()
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

  it('submits the goal and routine entered through their labelled fields', async () => {
    mockCreateLearningPath.mockResolvedValue({ learningPath: { id: 'new-id' } })
    render(<LearningPathForm onSave={jest.fn()} onCancel={jest.fn()} />)

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Learn Go' } })
    fireEvent.change(screen.getByLabelText('Goal'), { target: { value: 'Ship it' } })
    fireEvent.change(screen.getByLabelText('Recurring routine'), { target: { value: 'Daily' } })
    fireEvent.submit(screen.getByRole('button', { name: 'Save Learning Path' }).closest('form')!)

    await waitFor(() =>
      expect(mockCreateLearningPath).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Learn Go', goal: 'Ship it', routine: 'Daily' })
      )
    )
  })
})
