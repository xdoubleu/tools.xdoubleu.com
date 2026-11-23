import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import ChatAskCard from '@/components/factory/ChatAskCard'
import type { ChatItem } from '@/lib/factory/chat'

type Ask = Extract<ChatItem, { type: 'ask' }>

const ask = (over: Partial<Ask> = {}): Ask => ({
  type: 'ask',
  seq: 7,
  question: 'Which of A or B?',
  options: ['A', 'B'],
  multi: false,
  answer: null,
  ...over
})

describe('ChatAskCard', () => {
  it('answers with the option that was tapped', async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined)
    render(<ChatAskCard item={ask()} open onAnswer={onAnswer} />)
    expect(screen.getByText('Which of A or B?')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'B' }))
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith(7, 'B'))
  })

  it('answers with free text under "other"', async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined)
    render(<ChatAskCard item={ask()} open onAnswer={onAnswer} />)
    const answer = screen.getByRole('button', { name: 'Send other answer' })
    expect(answer).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Other answer'), { target: { value: ' neither ' } })
    fireEvent.click(answer)
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith(7, 'neither'))
  })

  it('collects several choices before sending them', async () => {
    const onAnswer = vi.fn().mockResolvedValue(undefined)
    render(<ChatAskCard item={ask({ multi: true })} open onAnswer={onAnswer} />)
    const send = screen.getByRole('button', { name: 'Send choices' })
    expect(send).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'A' }))
    fireEvent.click(screen.getByRole('button', { name: 'B' }))
    fireEvent.click(screen.getByRole('button', { name: 'A' }))
    expect(onAnswer).not.toHaveBeenCalled()
    fireEvent.click(send)
    await waitFor(() => expect(onAnswer).toHaveBeenCalledWith(7, 'B'))
  })

  it('says so when the answer did not go through, and lets the person retry', async () => {
    const onAnswer = vi.fn().mockRejectedValueOnce(new Error('no')).mockResolvedValue(undefined)
    render(<ChatAskCard item={ask()} open onAnswer={onAnswer} />)
    fireEvent.click(screen.getByRole('button', { name: 'A' }))
    expect(await screen.findByText(/didn't go through/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'A' }))
    await waitFor(() => expect(onAnswer).toHaveBeenCalledTimes(2))
  })

  it('shows the answer given and offers no options', () => {
    render(<ChatAskCard item={ask({ answer: 'B' })} open onAnswer={vi.fn()} />)
    expect(screen.getByText('B')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'A' })).not.toBeInTheDocument()
  })

  it('cannot be answered once the chat is closed', () => {
    render(<ChatAskCard item={ask()} open={false} onAnswer={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'A' })).toBeDisabled()
    expect(screen.getByText(/closed before this was answered/)).toBeInTheDocument()
  })
})
