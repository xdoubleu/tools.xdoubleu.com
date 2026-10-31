import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

vi.mock('@/hooks/useFactory', () => ({ useFactoryRepos: vi.fn() }))
vi.mock('@/components/factory/RepoDialog', () => ({
  __esModule: true,
  default: ({ repo, onClose }: { repo?: { name: string }; onClose: () => void }) => (
    <div data-testid="dialog" data-repo={repo?.name ?? 'new'} onClick={onClose} />
  )
}))

import ReposClient from '@/components/factory/ReposClient'
import { useFactoryRepos } from '@/hooks/useFactory'
import { Harness, ListReposResponseSchema, RepoSchema } from '@/lib/gen/factory/v1/factory_pb'

function mockRepos(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRepos).mockReturnValue({ isLoading: false, ...value })
}

const list = create(ListReposResponseSchema, {
  repos: [
    create(RepoSchema, {
      id: 'r1',
      owner: 'octo',
      name: 'cat',
      defaultBranch: 'main',
      defaultHarness: Harness.CLAUDE_CODE,
      timeLimitMinutes: 60
    }),
    create(RepoSchema, {
      id: 'r2',
      owner: 'octo',
      name: 'dog',
      defaultBranch: 'dev',
      defaultHarness: Harness.OPENCODE,
      timeLimitMinutes: 15
    })
  ]
})

beforeEach(() => vi.clearAllMocks())

describe('ReposClient', () => {
  it('lists each repo with its branch, harness and time limit', () => {
    mockRepos({ data: list })
    render(<ReposClient />)
    expect(screen.getByText('octo/cat')).toBeInTheDocument()
    expect(screen.getByText('main · Claude Code · 60 min')).toBeInTheDocument()
    expect(screen.getByText('octo/dog')).toBeInTheDocument()
    expect(screen.getByText('dev · OpenCode · 15 min')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Factory' })).toHaveAttribute('href', '/factory')
    expect(screen.getByText('Repos', { selector: '[aria-current="page"]' })).toBeInTheDocument()
  })

  it('shows loading, error and empty states', () => {
    mockRepos({ isLoading: true })
    const { unmount } = render(<ReposClient />)
    expect(screen.getByText(/Loading repos/)).toBeInTheDocument()
    unmount()

    mockRepos({ error: new Error('boom') })
    const second = render(<ReposClient />)
    expect(screen.getByText(/Failed to load repos/)).toBeInTheDocument()
    second.unmount()

    mockRepos({ data: create(ListReposResponseSchema, {}) })
    render(<ReposClient />)
    expect(screen.getByText('No repos yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows neither error nor spinner beside data', () => {
    mockRepos({ data: list, error: new Error('refetch'), isLoading: true })
    render(<ReposClient />)
    expect(screen.queryByText(/Failed to load/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading repos/)).not.toBeInTheDocument()
    expect(screen.queryByText('No repos yet.')).not.toBeInTheDocument()
  })

  it('opens the sheet to add a repo and closes it again', () => {
    mockRepos({ data: list })
    render(<ReposClient />)
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add repo' }))
    expect(screen.getByTestId('dialog')).toHaveAttribute('data-repo', 'new')

    fireEvent.click(screen.getByTestId('dialog'))
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('opens the sheet to edit the chosen repo', () => {
    mockRepos({ data: list })
    render(<ReposClient />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[1])
    expect(screen.getByTestId('dialog')).toHaveAttribute('data-repo', 'dog')
  })
})
