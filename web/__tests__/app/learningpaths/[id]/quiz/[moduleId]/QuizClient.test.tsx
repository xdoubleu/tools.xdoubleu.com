import React from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'

const mockMutate = jest.fn(async () => undefined)
const mockRecordItemProgress = jest.fn()

jest.mock('@/hooks/useLearningPaths', () => ({
  useLearningPath: jest.fn(),
  useRecordItemProgress: () => mockRecordItemProgress
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import QuizClient from '@/app/learningpaths/[id]/quiz/[moduleId]/QuizClient'
import { useLearningPath } from '@/hooks/useLearningPaths'
import { create } from '@bufbuild/protobuf'
import {
  type GetLearningPathResponse,
  GetLearningPathResponseSchema,
  ItemSchema,
  LearningPathSchema,
  ModuleSchema,
  QuizQuestionSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

function mockPath(state: { data?: GetLearningPathResponse; isLoading?: boolean; error?: Error }) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useLearningPath).mockReturnValue({
    data: undefined,
    isLoading: false,
    error: undefined,
    mutate: mockMutate,
    ...state
  })
}

function pathResponse() {
  const learningPath = create(LearningPathSchema, {
    id: 'lp1',
    title: 'Learn Go',
    modules: [
      create(ModuleSchema, {
        id: 'm1',
        title: 'Week 1',
        items: [create(ItemSchema, { id: 'c1', type: 'checkpoint', description: 'Quiz' })],
        quiz: [create(QuizQuestionSchema, { prompt: 'Q1', options: ['a', 'b'] })]
      }),
      create(ModuleSchema, {
        id: 'm2',
        title: 'Week 2',
        items: [create(ItemSchema, { id: 'i2', description: 'Do' })]
      })
    ]
  })
  return create(GetLearningPathResponseSchema, { learningPath })
}

beforeEach(() => jest.clearAllMocks())

describe('QuizClient', () => {
  it('shows loading state', () => {
    mockPath({ isLoading: true })
    render(<QuizClient id="lp1" moduleId="m1" />)
    expect(screen.getByText('Loading quiz…')).toBeInTheDocument()
  })

  it('shows error state', () => {
    mockPath({ error: new Error('fail') })
    render(<QuizClient id="lp1" moduleId="m1" />)
    expect(screen.getByText('Failed to load quiz.')).toBeInTheDocument()
  })

  it('renders the module quiz with a breadcrumb back to the path', () => {
    mockPath({ data: pathResponse() })
    render(<QuizClient id="lp1" moduleId="m1" />)
    expect(screen.getByRole('heading', { name: 'Quiz: Week 1' })).toBeInTheDocument()
    expect(screen.getByText('1. Q1')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Learn Go' })).toHaveAttribute(
      'href',
      '/learningpaths/lp1'
    )
    expect(screen.getByRole('link', { name: 'Back to path' })).toHaveAttribute(
      'href',
      '/learningpaths/lp1'
    )
  })

  it('records the checkpoint complete when the quiz is passed', async () => {
    mockPath({ data: pathResponse() })
    render(<QuizClient id="lp1" moduleId="m1" />)
    fireEvent.click(screen.getByRole('button', { name: /^a$/ }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Check answers' }))
    })
    expect(mockRecordItemProgress).toHaveBeenCalledWith({ itemId: 'c1', completed: true })
    expect(mockMutate).toHaveBeenCalled()
  })

  it.each(['m2', 'missing'])('shows an empty state for module %s without a quiz', (moduleId) => {
    mockPath({ data: pathResponse() })
    render(<QuizClient id="lp1" moduleId={moduleId} />)
    expect(screen.getByText('This module has no quiz.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to path' })).toBeInTheDocument()
  })
})
