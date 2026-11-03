import { create } from '@bufbuild/protobuf'
import { JobEventSchema } from '@/lib/gen/factory/v1/factory_pb'
import { logEntries } from '@/lib/factory/joblog'

const event = (seq: number, payload: unknown) =>
  create(JobEventSchema, {
    seq,
    kind: 'x',
    payload: typeof payload === 'string' ? payload : JSON.stringify(payload)
  })

describe('logEntries', () => {
  it('shows what the agent says and the tools it calls', () => {
    const entries = logEntries(
      event(4, {
        type: 'assistant',
        message: {
          content: [
            { type: 'text', text: 'Reading the file.' },
            { type: 'tool_use', name: 'Bash', input: { command: 'ls -la' } },
            { type: 'tool_use', input: { file_path: '/a/b.go' } },
            { type: 'tool_use', name: 'Odd', input: { other: 1 } },
            { type: 'tool_use', name: 'Bare', input: 'nope' },
            { type: 'tool_use', name: 'Idle', input: {} },
            { type: 'text', text: '' },
            'not a block'
          ]
        }
      })
    )
    expect(entries).toEqual([
      { key: '4.0', kind: 'text', title: 'Agent', body: 'Reading the file.' },
      { key: '4.1', kind: 'tool', title: 'Bash', body: 'ls -la' },
      { key: '4.2', kind: 'tool', title: 'Tool', body: '/a/b.go' },
      { key: '4.3', kind: 'tool', title: 'Odd', body: '{"other":1}' },
      { key: '4.4', kind: 'tool', title: 'Bare', body: '' },
      { key: '4.5', kind: 'tool', title: 'Idle', body: '' }
    ])
  })

  it('reads a plain-string message and ignores an empty or odd one', () => {
    const say = (message: unknown, type = 'assistant') => logEntries(event(1, { type, message }))
    expect(say({ content: 'Done.' })).toEqual([
      { key: '1.0', kind: 'text', title: 'Agent', body: 'Done.' }
    ])
    expect(say({ content: '' })).toEqual([])
    expect(say({ content: 'echo' }, 'user')).toEqual([])
    expect(say({ content: 7 })).toEqual([])
    expect(say('text')).toEqual([])
    expect(say({ content: [{ type: 'text', text: 'user text' }] }, 'user')).toEqual([])
  })

  it('shows tool output, as text blocks or a string, and marks errors', () => {
    const entries = logEntries(
      event(5, {
        type: 'user',
        message: {
          content: [
            { type: 'tool_result', content: 'plain output' },
            {
              type: 'tool_result',
              content: [{ type: 'text', text: 'a' }, { type: 'image' }, { type: 'text', text: 'b' }]
            },
            { type: 'tool_result', content: 'boom', is_error: true },
            { type: 'tool_result', content: 12 }
          ]
        }
      })
    )
    expect(entries.map((e) => [e.kind, e.title, e.body])).toEqual([
      ['note', 'Tool output', 'plain output'],
      ['note', 'Tool output', 'a\nb'],
      ['error', 'Tool error', 'boom'],
      ['note', 'Tool output', '']
    ])
  })

  it('shows how the run ended', () => {
    expect(logEntries(event(9, { type: 'result', result: 'All done' }))).toEqual([
      { key: '9.0', kind: 'result', title: 'Finished', body: 'All done' }
    ])
    expect(logEntries(event(9, { type: 'result', is_error: true, result: 'Nope' }))).toEqual([
      { key: '9.0', kind: 'error', title: 'Failed', body: 'Nope' }
    ])
  })

  it('shows output that is not JSON as it came, and skips what the log has no use for', () => {
    expect(logEntries(event(2, '{"type":"assistant","message":{"con'))).toEqual([
      { key: '2.0', kind: 'note', title: 'Output', body: '{"type":"assistant","message":{"con' }
    ])
    expect(logEntries(event(1, { type: 'system', subtype: 'init' }))).toEqual([])
    expect(logEntries(event(1, [1, 2]))).toEqual([])
    expect(logEntries(event(1, 'null'))).toEqual([])
  })

  it('cuts a very long body', () => {
    const [entry] = logEntries(event(1, { type: 'result', result: 'x'.repeat(5000) }))
    expect(entry?.body).toHaveLength(4001)
    expect(entry?.body.endsWith('…')).toBe(true)
    const [exact] = logEntries(event(1, { type: 'result', result: 'x'.repeat(4000) }))
    expect(exact?.body).toHaveLength(4000)
  })
})
