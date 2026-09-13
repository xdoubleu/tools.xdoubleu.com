import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useLearningPaths', () => ({
  useLearningPaths: jest.fn(),
  useFetchLearningPathsPage: jest.fn(() => jest.fn())
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import LearningPathsListClient from '@/components/learningpaths/LearningPathsListClient'
import { useLearningPaths } from '@/hooks/useLearningPaths'
import { create } from '@bufbuild/protobuf'
import {
  LearningPathSchema,
  ModuleSchema,
  ItemSchema,
  ListLearningPathsResponseSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import type { ListLearningPathsResponse } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

function mockLearningPaths(value: {
  data?: ListLearningPathsResponse
  error?: Error
  isLoading: boolean
}) {
  jest.mocked(useLearningPaths).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: jest.fn(async () => undefined)
  })
}

beforeEach(() => jest.clearAllMocks())

describe('LearningPathsListClient', () => {
  it('shows a loading state', () => {
    mockLearningPaths({ isLoading: true })
    render(<LearningPathsListClient />)
    expect(screen.getByText('Loading learning paths…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockLearningPaths({ error: new Error('boom'), isLoading: false })
    render(<LearningPathsListClient />)
    expect(screen.getByText('Failed to load learning paths.')).toBeInTheDocument()
  })

  it('shows an empty state', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, { learningPaths: [] }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    expect(screen.getByText('No learning paths yet. Create your first one!')).toBeInTheDocument()
  })

  it('renders learning path cards with goal and module/item counts', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, {
        learningPaths: [
          create(LearningPathSchema, {
            id: 'lp1',
            title: 'Learn Go',
            goal: 'Ship a backend service',
            modules: [
              create(ModuleSchema, {
                title: 'Month 1',
                items: [
                  create(ItemSchema, { description: 'Read', completed: true }),
                  create(ItemSchema, { description: 'Write', completed: false })
                ]
              })
            ]
          })
        ]
      }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    const link = screen.getByRole('link', { name: /Learn Go/ })
    expect(link).toHaveAttribute('href', '/learningpaths/lp1')
    expect(link).toHaveTextContent('Ship a backend service')
    expect(link).toHaveTextContent('1 module')
    expect(link).toHaveTextContent('1/2 done')
  })
})
