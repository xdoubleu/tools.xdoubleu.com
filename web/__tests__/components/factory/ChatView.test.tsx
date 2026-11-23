import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ChatSocket } from '@/hooks/useChatSocket'

let socket: ChatSocket
vi.mock('@/hooks/useChatSocket', () => ({ useChatSocket: () => socket }))
const actions = {
  send: vi.fn(),
  answer: vi.fn(),
  close: vi.fn()
}
vi.mock('@/hooks/useFactory', () => ({ useChatActions: () => actions }))

import ChatView from '@/components/factory/ChatView'

function setSocket(state: string, items: ChatSocket['transcript']['items'] = [], connected = true) {
  socket = { state, connected, transcript: { items, cursor: items.length } }
}

const renderView = (chat?: { state: string; report: string }) =>
  // @ts-expect-error -- partial Chat
  render(<ChatView id="c1" chat={chat} onLeave={vi.fn()} />)

beforeEach(() => {
  vi.clearAllMocks()
  actions.send.mockResolvedValue({})
  actions.answer.mockResolvedValue({})
})

describe('ChatView', () => {
  it('shows starting until the runner attaches', () => {
    setSocket('', [], false)
    renderView()
    expect(screen.getByText('Starting…')).toBeInTheDocument()
    expect(screen.getByText('Reconnecting…')).toBeInTheDocument()
    expect(screen.getByLabelText('Message')).toBeDisabled()
  })

  it('renders markdown, a collapsed tool line, the plan and errors', () => {
    setSocket('working', [
      { type: 'agent', seq: 1, text: '**bold** text' },
      {
        type: 'tool',
        seq: 2,
        id: 't',
        title: 'Read',
        kind: '',
        status: 'completed',
        detail: 'a.go'
      },
      {
        type: 'plan',
        seq: 3,
        entries: [
          { content: 'one', status: 'completed' },
          { content: 'two', status: 'in_progress' },
          { content: 'three', status: 'pending' }
        ]
      },
      { type: 'user', seq: 4, text: 'hello', answer: true },
      { type: 'error', seq: 5, text: 'boom' }
    ])
    renderView()
    expect(screen.getByText('bold').tagName).toBe('STRONG')
    expect(screen.getByRole('button', { name: /Read · a.go/ })).toBeInTheDocument()
    expect(screen.getByLabelText('Plan').children).toHaveLength(3)
    expect(screen.getByText('hello')).toBeInTheDocument()
    expect(screen.getByText('boom')).toBeInTheDocument()
    expect(screen.getByText('Working')).toBeInTheDocument()
    expect(screen.getByLabelText('Message')).toBeDisabled()
  })

  it('sends a message when idle', async () => {
    setSocket('idle')
    renderView()
    expect(screen.getByText('Ready. Say what you want done.')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Message'), { target: { value: ' read it ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(actions.send).toHaveBeenCalledWith('c1', 'read it'))
    await waitFor(() => expect(screen.getByLabelText('Message')).toHaveValue(''))
  })

  it('answers when asked, and reports a failure', async () => {
    setSocket('asking', [])
    actions.answer.mockRejectedValueOnce(new Error('no'))
    renderView()
    fireEvent.change(screen.getByLabelText('Your answer'), { target: { value: 'yes' } })
    fireEvent.click(screen.getByRole('button', { name: 'Answer' }))
    expect(await screen.findByText(/didn't go through/)).toBeInTheDocument()
    expect(actions.answer).toHaveBeenCalledWith('c1', 'yes', 0)
  })

  it('answers a question card with its id, and the composer answers the pending card', async () => {
    setSocket('asking', [
      {
        type: 'ask',
        seq: 4,
        question: 'A or B?',
        options: ['A', 'B'],
        multi: false,
        answer: null
      }
    ])
    renderView()
    fireEvent.click(screen.getByRole('button', { name: 'B' }))
    await waitFor(() => expect(actions.answer).toHaveBeenCalledWith('c1', 'B', 4))
    fireEvent.change(screen.getByLabelText('Your answer'), { target: { value: 'neither' } })
    fireEvent.click(screen.getByRole('button', { name: 'Answer' }))
    await waitFor(() => expect(actions.answer).toHaveBeenCalledWith('c1', 'neither', 4))
  })

  it('closes after confirming and shows the leftover report', async () => {
    setSocket('idle')
    actions.close.mockResolvedValue({ report: 'Nothing was left on branch x.' })
    renderView()
    fireEvent.click(screen.getByRole('button', { name: 'Close chat' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Close chat', hidden: false }))
    await waitFor(() => expect(actions.close).toHaveBeenCalledWith('c1'))
  })

  it('keeps the dialog and says so when closing fails', async () => {
    setSocket('idle')
    actions.close.mockRejectedValue(new Error('no'))
    renderView()
    fireEvent.click(screen.getByRole('button', { name: 'Close chat' }))
    const buttons = await screen.findAllByRole('button', { name: 'Close chat' })
    fireEvent.click(buttons[buttons.length - 1]!)
    expect(await screen.findByText(/didn't go through/)).toBeInTheDocument()
  })

  it('shows the report of a closed chat from the list', () => {
    setSocket('stopped')
    renderView({ state: 'stopped', report: 'Branch b was left with 1 uncommitted file(s).' })
    expect(screen.getByTestId('chat-report')).toHaveTextContent('left with 1 uncommitted')
    expect(screen.queryByLabelText('Message')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Close chat' })).not.toBeInTheDocument()
  })

  it('falls back when a closed chat has no report', () => {
    setSocket('stopped')
    renderView()
    expect(screen.getByText('No report was recorded.')).toBeInTheDocument()
  })

  it('stays at the bottom as the page grows, unless scrolled up', () => {
    let onResize = () => {}
    const disconnect = vi.fn()
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(cb: () => void) {
          onResize = cb
        }
        observe() {}
        disconnect = disconnect
      }
    )
    const scroll = vi.fn()
    const original = Element.prototype.scrollIntoView
    Element.prototype.scrollIntoView = scroll
    const root = document.documentElement
    const height = vi.spyOn(root, 'scrollHeight', 'get').mockReturnValue(window.innerHeight)
    setSocket('idle')
    const { unmount } = renderView()
    scroll.mockClear()

    height.mockReturnValue(window.innerHeight + 500)
    onResize()
    expect(scroll).toHaveBeenCalledTimes(1)

    onResize()
    expect(scroll).toHaveBeenCalledTimes(1)

    unmount()
    expect(disconnect).toHaveBeenCalled()
    height.mockRestore()
    Element.prototype.scrollIntoView = original
    vi.unstubAllGlobals()
  })
})
