import { fireEvent, render, screen, waitFor } from '@testing-library/react'

const mockPush = vi.fn()
let params = new URLSearchParams()
vi.mock('@/hooks/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => params
}))
vi.mock('@/components/AppSections', () => ({ __esModule: true, default: () => null }))
vi.mock('@/components/factory/ChatView', () => ({
  __esModule: true,
  default: ({ id, chat, onLeave }: { id: string; chat?: { id: string }; onLeave: () => void }) => (
    <button onClick={onLeave}>
      view {id} {chat?.id ?? 'none'}
    </button>
  )
}))
const mockStart = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryChats: vi.fn(),
  useFactoryRepos: vi.fn(),
  useFactoryRunners: vi.fn(),
  useChatActions: () => ({ start: mockStart })
}))

import { create } from '@bufbuild/protobuf'
import {
  ChatSchema,
  ListChatsResponseSchema,
  ListReposResponseSchema,
  RepoSchema
} from '@/lib/gen/factory/v1/factory_pb'
import { ListRunnersResponseSchema, RunnerSchema } from '@/lib/gen/factory/v1/runners_pb'
import ChatClient from '@/components/factory/ChatClient'
import { useFactoryChats, useFactoryRepos, useFactoryRunners } from '@/hooks/useFactory'

function setup(
  opts: {
    chats?: NonNullable<Parameters<typeof create<typeof ChatSchema>>[1]>[]
    online?: boolean
    error?: boolean
  } = {}
) {
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryChats).mockReturnValue({
    data: opts.error ? undefined : create(ListChatsResponseSchema, { chats: opts.chats ?? [] }),
    error: opts.error ? new Error('x') : undefined,
    isLoading: false
  })
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRepos).mockReturnValue({
    data: create(ListReposResponseSchema, {
      repos: [create(RepoSchema, { id: 'r1', owner: 'o', name: 'n' })]
    })
  })
  // @ts-expect-error -- partial SWRResponse
  vi.mocked(useFactoryRunners).mockReturnValue({
    data: create(ListRunnersResponseSchema, {
      runners: [create(RunnerSchema, { id: 'run1', online: opts.online ?? true })]
    })
  })
}

const chat = (fields: NonNullable<Parameters<typeof create<typeof ChatSchema>>[1]>) =>
  create(ChatSchema, fields)

beforeEach(() => {
  params = new URLSearchParams()
  mockPush.mockClear()
  mockStart.mockReset()
})

describe('ChatClient', () => {
  it('starts a chat with OpenCode by default and opens it', async () => {
    setup()
    mockStart.mockResolvedValue({ id: 'c9' })
    render(<ChatClient />)
    expect(screen.getByLabelText('Harness')).toHaveValue('2')
    expect(screen.getByText('No chats yet.')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Harness'), { target: { value: '1' } })
    fireEvent.change(screen.getByLabelText('Repo'), { target: { value: 'r1' } })
    fireEvent.click(screen.getByRole('button', { name: 'Start chat' }))
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith('/factory/chat?chat=c9'))
    expect(mockStart).toHaveBeenCalledWith('r1', 1, 'run1')
  })

  it('says when the start fails', async () => {
    setup()
    mockStart.mockRejectedValue(new Error('no'))
    render(<ChatClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Start chat' }))
    expect(await screen.findByText(/Couldn't start the chat/)).toBeInTheDocument()
  })

  it('cannot start without an online runner', () => {
    setup({ online: false })
    render(<ChatClient />)
    expect(screen.getByText('No runner is online to host a chat.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Start chat' })).toBeDisabled()
  })

  it('lists chats and resumes one', () => {
    setup({
      chats: [
        chat({ id: 'c1', repoId: 'r1', harness: 2, state: 'idle' }),
        chat({ id: 'c2', repoId: 'gone', harness: 1, state: 'stopped' })
      ]
    })
    render(<ChatClient />)
    fireEvent.click(screen.getByRole('button', { name: 'Resume' }))
    expect(mockPush).toHaveBeenCalledWith('/factory/chat?chat=c1')
    expect(screen.getByText('View')).toBeInTheDocument()
    expect(screen.getAllByText('Claude Code').length).toBeGreaterThan(1)
  })

  it('filters the list by repo and shows a paused chat', () => {
    setup({
      chats: [
        chat({ id: 'c1', repoId: 'r1', harness: 2, state: 'paused', taskId: 't1' }),
        chat({ id: 'c2', repoId: 'gone', harness: 2, state: 'idle' })
      ]
    })
    render(<ChatClient />)
    expect(screen.getByText('Paused')).toBeInTheDocument()
    expect(screen.getByText(/linked to a task/)).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Filter by repo'), { target: { value: 'r1' } })
    expect(screen.queryByText('Ready')).not.toBeInTheDocument()
    expect(screen.getByText('Paused')).toBeInTheDocument()
  })

  it('says so when no chat is in the filtered repo', () => {
    setup({ chats: [chat({ id: 'c2', repoId: 'gone', harness: 2, state: 'idle' })] })
    render(<ChatClient />)
    fireEvent.change(screen.getByLabelText('Filter by repo'), { target: { value: 'r1' } })
    expect(screen.getByText('No chats for this repo.')).toBeInTheDocument()
  })

  it('shows an error loading chats', () => {
    setup({ error: true })
    render(<ChatClient />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('shows a chat when one is selected, and leaves it', () => {
    setup({ chats: [chat({ id: 'c1' })] })
    params = new URLSearchParams('chat=c1')
    render(<ChatClient />)
    fireEvent.click(screen.getByRole('button', { name: 'view c1 c1' }))
    expect(mockPush).toHaveBeenCalledWith('/factory/chat')
  })
})
