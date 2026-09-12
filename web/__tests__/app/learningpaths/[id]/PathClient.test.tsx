import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockMutate = jest.fn(async () => undefined)
const mockDeleteLearningPath = jest.fn()
const mockRecordItemProgress = jest.fn()

jest.mock('@/hooks/useLearningPaths', () => ({
  useLearningPath: jest.fn(),
  useDeleteLearningPath: () => mockDeleteLearningPath,
  useRecordItemProgress: () => mockRecordItemProgress
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import PathClient from '@/app/learningpaths/[id]/PathClient'
import { useLearningPath } from '@/hooks/useLearningPaths'
import { useRouter } from 'next/navigation'
import { create } from '@bufbuild/protobuf'
import {
  LearningPathSchema,
  ModuleSchema,
  ItemSchema,
  ResourceSchema,
  GetLearningPathResponseSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

const mockRouter = { push: jest.fn() }

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- partial mock
  jest.mocked(useRouter).mockReturnValue(mockRouter)
})

describe('PathClient', () => {
  it('shows loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined,
      mutate: mockMutate
    })
    render(<PathClient id="lp1" />)
    expect(screen.getByText('Loading learning path…')).toBeInTheDocument()
  })

  it('shows error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('fail'),
      mutate: mockMutate
    })
    render(<PathClient id="lp1" />)
    expect(screen.getByText('Failed to load learning path.')).toBeInTheDocument()
  })

  it('renders modules, items, resources and progress count', () => {
    const learningPath = create(LearningPathSchema, {
      id: 'lp1',
      title: 'Learn Go',
      goal: 'Ship it',
      routine: 'Daily',
      modules: [
        create(ModuleSchema, {
          id: 'm1',
          title: 'Month 1',
          items: [
            create(ItemSchema, { id: 'i1', type: 'read', description: 'Read', completed: true }),
            create(ItemSchema, { id: 'i2', description: 'Write', completed: false })
          ]
        })
      ],
      resources: [create(ResourceSchema, { id: 'r1', text: 'https://go.dev' })]
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath }),
      isLoading: false,
      error: undefined,
      mutate: mockMutate
    })
    render(<PathClient id="lp1" />)
    expect(screen.getByRole('heading', { name: 'Learn Go' })).toBeInTheDocument()
    expect(screen.getByText('Ship it')).toBeInTheDocument()
    expect(screen.getByText('Routine: Daily')).toBeInTheDocument()
    expect(screen.getByText('1/2 items complete')).toBeInTheDocument()
    expect(screen.getByText('Read')).toBeInTheDocument()
    expect(screen.getByText('Write')).toBeInTheDocument()
    expect(screen.getByText('https://go.dev')).toBeInTheDocument()
    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes[0]).toBeChecked()
    expect(checkboxes[1]).not.toBeChecked()
  })

  it('records progress and revalidates when a checkbox is toggled', async () => {
    const learningPath = create(LearningPathSchema, {
      id: 'lp1',
      title: 'Learn Go',
      modules: [
        create(ModuleSchema, {
          id: 'm1',
          title: 'Month 1',
          items: [create(ItemSchema, { id: 'i1', description: 'Read', completed: false })]
        })
      ]
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath }),
      isLoading: false,
      error: undefined,
      mutate: mockMutate
    })
    render(<PathClient id="lp1" />)

    fireEvent.click(screen.getByRole('checkbox'))

    await waitFor(() => {
      expect(mockRecordItemProgress).toHaveBeenCalledWith({ itemId: 'i1', completed: true })
      expect(mockMutate).toHaveBeenCalled()
    })
  })

  it('deletes the learning path after confirming and navigates back to the list', async () => {
    const learningPath = create(LearningPathSchema, { id: 'lp1', title: 'Learn Go' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath }),
      isLoading: false,
      error: undefined,
      mutate: mockMutate
    })
    mockDeleteLearningPath.mockResolvedValue({})
    render(<PathClient id="lp1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirm delete' }))

    await waitFor(() => {
      expect(mockDeleteLearningPath).toHaveBeenCalledWith({ id: 'lp1' })
      expect(mockRouter.push).toHaveBeenCalledWith('/learningpaths/list')
    })
  })

  it('cancels the delete confirmation', () => {
    const learningPath = create(LearningPathSchema, { id: 'lp1', title: 'Learn Go' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath }),
      isLoading: false,
      error: undefined,
      mutate: mockMutate
    })
    render(<PathClient id="lp1" />)

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
    expect(mockDeleteLearningPath).not.toHaveBeenCalled()
  })
})
