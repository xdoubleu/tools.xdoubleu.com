import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useLearningPaths', () => ({
  useLearningPath: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/learningpaths/LearningPathForm', () => {
  return function MockLearningPathForm() {
    return <div data-testid="learning-path-form">learning-path-form-mock</div>
  }
})

import EditLearningPathClient from '@/app/learningpaths/[id]/edit/EditLearningPathClient'
import { useLearningPath } from '@/hooks/useLearningPaths'
import { useRouter } from 'next/navigation'
import { create } from '@bufbuild/protobuf'
import {
  LearningPathSchema,
  GetLearningPathResponseSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

const mockRouter = { push: jest.fn() }
const mockLearningPath = create(LearningPathSchema, { id: 'lp-1', title: 'Learn Go' })

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock router returns partial AppRouterInstance
  jest.mocked(useRouter).mockReturnValue(mockRouter)
})

describe('EditLearningPathClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<EditLearningPathClient id="lp-1" />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('renders LearningPathForm when the path is loaded', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath: mockLearningPath }),
      isLoading: false,
      error: undefined
    })

    render(<EditLearningPathClient id="lp-1" />)
    expect(screen.getByTestId('learning-path-form')).toBeInTheDocument()
  })

  it('calls useLearningPath with the provided id', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<EditLearningPathClient id="lp-123" />)
    expect(useLearningPath).toHaveBeenCalledWith('lp-123')
  })

  it('renders back link with correct href', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: create(GetLearningPathResponseSchema, { learningPath: mockLearningPath }),
      isLoading: false,
      error: undefined
    })

    render(<EditLearningPathClient id="lp-1" />)
    const backLink = screen.getByRole('link', { name: 'Learn Go' })
    expect(backLink).toHaveAttribute('href', '/learningpaths/lp-1')
  })

  it('renders page title', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLearningPath).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<EditLearningPathClient id="lp-1" />)
    expect(screen.getByText('Edit Learning Path')).toBeInTheDocument()
  })
})
