import {
  applyFrame,
  chatSocketUrl,
  EMPTY_TRANSCRIPT,
  isChatFrame,
  stateLabel,
  toolLine,
  type ChatFrame,
  type ChatTranscript
} from '@/lib/factory/chat'

const frame = (seq: number, kind: string, payload: unknown): ChatFrame => ({
  seq,
  kind,
  payload: typeof payload === 'string' ? payload : JSON.stringify(payload),
  createdAt: ''
})
const update = (seq: number, u: unknown) => frame(seq, 'update', u)
const fold = (frames: ChatFrame[]): ChatTranscript => frames.reduce(applyFrame, EMPTY_TRANSCRIPT)

describe('applyFrame', () => {
  it('joins message chunks into one agent item', () => {
    const t = fold([
      update(1, { sessionUpdate: 'agent_message_chunk', content: { text: 'Hel' } }),
      update(2, { sessionUpdate: 'agent_message_chunk', content: { text: 'lo' } })
    ])
    expect(t.items).toEqual([{ type: 'agent', seq: 1, text: 'Hello' }])
    expect(t.cursor).toBe(2)
  })

  it('drops a seq it already has', () => {
    const f = update(1, { sessionUpdate: 'agent_message_chunk', content: { text: 'a' } })
    expect(fold([f, f]).items).toHaveLength(1)
  })

  it('shows user messages and answers', () => {
    const t = fold([frame(1, 'user_message', 'hi'), frame(2, 'user_answer', 'yes')])
    expect(t.items).toEqual([
      { type: 'user', seq: 1, text: 'hi', answer: false },
      { type: 'user', seq: 2, text: 'yes', answer: true }
    ])
  })

  it('tracks a tool call through its updates', () => {
    const t = fold([
      update(1, {
        sessionUpdate: 'tool_call',
        toolCallId: 't1',
        title: 'Read',
        locations: [{ path: 'a.go' }]
      }),
      update(2, { sessionUpdate: 'tool_call_update', toolCallId: 't1', status: 'completed' }),
      update(3, { sessionUpdate: 'tool_call_update', toolCallId: 'other', status: 'failed' })
    ])
    const tool = t.items[0]
    expect(tool).toMatchObject({ type: 'tool', status: 'completed', detail: 'a.go' })
    if (tool?.type === 'tool') expect(toolLine(tool)).toBe('Read · a.go')
  })

  it('reads a tool target from raw input and appends output', () => {
    const t = fold([
      update(1, {
        sessionUpdate: 'tool_call',
        toolCallId: 't1',
        rawInput: { command: 'ls' }
      }),
      update(2, {
        sessionUpdate: 'tool_call_update',
        toolCallId: 't1',
        title: 'Bash',
        rawOutput: 'x'
      })
    ])
    expect(t.items[0]).toMatchObject({ title: 'Bash' })
    expect(t.items[0]).toMatchObject({ detail: expect.stringContaining('x') })
  })

  it('replaces the plan in place', () => {
    const t = fold([
      update(1, { sessionUpdate: 'plan', entries: [{ content: 'a', status: 'pending' }] }),
      update(2, { sessionUpdate: 'agent_message_chunk', content: { text: 'x' } }),
      update(3, { sessionUpdate: 'plan', entries: [{ content: 'a', status: 'completed' }, 5] })
    ])
    expect(t.items[0]).toEqual({
      type: 'plan',
      seq: 3,
      entries: [{ content: 'a', status: 'completed' }]
    })
    expect(t.items).toHaveLength(2)
  })

  it('records errors, and ignores other kinds but advances the cursor', () => {
    const t = fold([
      frame(1, 'error', 'boom'),
      frame(2, 'permission', {}),
      update(3, { sessionUpdate: 'agent_thought_chunk' }),
      update(4, 'not json')
    ])
    expect(t.items).toEqual([{ type: 'error', seq: 1, text: 'boom' }])
    expect(t.cursor).toBe(4)
  })
})

describe('helpers', () => {
  it('labels states', () => {
    expect(['working', 'asking', 'idle', 'stopped', ''].map(stateLabel)).toEqual([
      'Working',
      'Waiting for your answer',
      'Ready',
      'Closed',
      'Starting'
    ])
  })

  it('builds the socket url', () => {
    expect(chatSocketUrl('https://x.test/', 'c1', 4)).toBe(
      'wss://x.test/factory/chats/c1/ws?after=4'
    )
    expect(chatSocketUrl('http://localhost:8080', 'c1', 0)).toBe(
      'ws://localhost:8080/factory/chats/c1/ws?after=0'
    )
  })

  it('recognises frames', () => {
    expect(isChatFrame({ seq: 1, kind: 'update' })).toBe(true)
    expect(isChatFrame({ seq: '1' })).toBe(false)
    expect(isChatFrame(null)).toBe(false)
  })
})
