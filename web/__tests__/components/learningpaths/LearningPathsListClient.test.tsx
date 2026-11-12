import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/hooks/useLearningPaths', () => ({
  useLearningPaths: vi.fn(),
  useFetchLearningPathsPage: vi.fn(() => vi.fn())
}))

import LearningPathsListClient from '@/components/learningpaths/LearningPathsListClient'
import { useLearningPaths } from '@/hooks/useLearningPaths'
import { create } from '@bufbuild/protobuf'
import {
  LearningPathSchema,
  ListLearningPathsResponseSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import type { ListLearningPathsResponse } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

function mockLearningPaths(value: {
  data?: ListLearningPathsResponse
  error?: Error
  isLoading: boolean
}) {
  vi.mocked(useLearningPaths).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: vi.fn(async () => undefined)
  })
}

beforeEach(() => vi.clearAllMocks())

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

  it('links to settings, where Todoist is connected and reconnected', () => {
    mockLearningPaths({ isLoading: true })
    render(<LearningPathsListClient />)
    expect(screen.getByRole('link', { name: /settings/i })).toHaveAttribute(
      'href',
      '/learningpaths/settings'
    )
  })

  it('renders learning path cards with goal and module/item counts', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, {
        learningPaths: [
          create(LearningPathSchema, {
            id: 'lp1',
            title: 'Learn Go',
            goal: 'Ship a backend service',
            moduleCount: 1,
            itemCount: 2,
            completedItemCount: 1
          })
        ]
      }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    const link = screen.getByRole('link', { name: /Learn Go/ })
    expect(link).toHaveAttribute('href', '/learningpaths/lp1')
    expect(link).toHaveTextContent('Ship a backend service')
    expect(link).toHaveTextContent('Ship a backend service · 1 module')
  })

  it('pluralises modules and omits progress for a path without items', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, {
        learningPaths: [
          create(LearningPathSchema, {
            id: 'lp2',
            title: 'Empty plan',
            moduleCount: 2
          })
        ]
      }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    const link = screen.getByRole('link', { name: /Empty plan/ })
    expect(link.textContent).toBe('Empty plan2 modules')
  })

  it('shows progress as a bar and leaves editing to the detail page', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, {
        learningPaths: [
          create(LearningPathSchema, {
            id: 'lp1',
            title: 'Learn Go',
            moduleCount: 1,
            itemCount: 2,
            completedItemCount: 1
          })
        ]
      }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    expect(screen.getByRole('progressbar', { name: 'Learn Go progress' })).toHaveAttribute(
      'aria-valuetext',
      '1 of 2 done'
    )
    expect(screen.queryByRole('link', { name: 'Edit' })).not.toBeInTheDocument()
  })

  it('shows no bar for a path without items', () => {
    mockLearningPaths({
      data: create(ListLearningPathsResponseSchema, {
        learningPaths: [create(LearningPathSchema, { id: 'lp1', title: 'Learn Go' })]
      }),
      isLoading: false
    })
    render(<LearningPathsListClient />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })
})
