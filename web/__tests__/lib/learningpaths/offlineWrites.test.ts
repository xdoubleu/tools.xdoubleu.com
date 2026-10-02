import { create, isMessage, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetLearningPathResponseSchema,
  ListLearningPathsResponseSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import {
  learningPathWrites,
  recordItemProgressWrite,
  setPausedWrite
} from '@/lib/learningpaths/offlineWrites'

function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  key: unknown,
  data: unknown
) {
  return write.apply(key, data, write.encode(init), undefined)
}

const path = (id: string) => ({
  id,
  modules: [
    {
      id: `${id}-m`,
      items: [
        { id: `${id}-1`, completed: false },
        { id: `${id}-2`, completed: false, linkedBookId: 'book-1' }
      ]
    }
  ]
})

const detail = () => create(GetLearningPathResponseSchema, { learningPath: path('a') })
const list = () =>
  create(ListLearningPathsResponseSchema, { learningPaths: [path('a'), path('b')], hasMore: true })

const items = (data: unknown) =>
  isMessage(data, GetLearningPathResponseSchema) ? (data.learningPath?.modules[0].items ?? []) : []

describe('learning path offline writes', () => {
  it('leave unrelated keys and data untouched', () => {
    const data = detail()
    for (const write of learningPathWrites) {
      expect(write.apply('/recipes', data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(setPausedWrite, { id: 'a' }, swrKeys.learningPath('a'), 'nope')).toBe('nope')
    const empty = create(GetLearningPathResponseSchema, {})
    expect(run(setPausedWrite, { id: 'a' }, swrKeys.learningPath('a'), empty)).toBe(empty)
    expect(run(setPausedWrite, { id: 'a', paused: true }, swrKeys.learningPaths, data)).toBe(data)
    expect(run(setPausedWrite, { id: 'a' }, ['/learningpaths/a'], data)).toBe(data)
    const paths = list()
    expect(run(setPausedWrite, { id: 'a', paused: true }, '/learningpaths/other', paths)).toBe(
      paths
    )
  })

  it('ticks an item off in its path, leaving book-linked items alone', () => {
    const done = run(
      recordItemProgressWrite,
      { itemId: 'a-1', completed: true },
      swrKeys.learningPath('a'),
      detail()
    )
    expect(items(done).map((i) => i.completed)).toEqual([true, false])

    const linked = run(
      recordItemProgressWrite,
      { itemId: 'a-2', completed: true },
      swrKeys.learningPath('a'),
      detail()
    )
    expect(items(linked).map((i) => i.completed)).toEqual([false, false])

    const reopened = run(
      recordItemProgressWrite,
      { itemId: 'a-1', completed: false },
      swrKeys.learningPath('a'),
      done
    )
    expect(items(reopened)[0].completed).toBe(false)
  })

  it('updates the list too', () => {
    const result = run(
      recordItemProgressWrite,
      { itemId: 'b-1', completed: true },
      swrKeys.learningPaths,
      list()
    )
    expect(result).toMatchObject({
      hasMore: true,
      learningPaths: [
        { id: 'a', modules: [{ items: [{ completed: false }, {}] }] },
        { id: 'b', modules: [{ items: [{ completed: true }, {}] }] }
      ]
    })
  })

  it('pauses and resumes only the given path', () => {
    expect(
      run(setPausedWrite, { id: 'b', paused: true }, swrKeys.learningPaths, list())
    ).toMatchObject({
      learningPaths: [
        { id: 'a', paused: false },
        { id: 'b', paused: true }
      ]
    })
    expect(
      run(setPausedWrite, { id: 'a', paused: true }, swrKeys.learningPath('a'), detail())
    ).toMatchObject({
      learningPath: { paused: true }
    })
  })

  it('describes each write', () => {
    const describe = <I extends DescMessage>(w: OfflineWrite<I>, init: MessageInitShape<I>) =>
      w.describe(w.encode(init))
    expect(describe(recordItemProgressWrite, { completed: true })).toBe('Complete a learning item')
    expect(describe(recordItemProgressWrite, {})).toBe('Reopen a learning item')
    expect(describe(setPausedWrite, { paused: true })).toBe('Pause a learning path')
    expect(describe(setPausedWrite, {})).toBe('Resume a learning path')
  })

  it('refetches learning paths and registers every write', () => {
    expect(learningPathWrites).toEqual([recordItemProgressWrite, setPausedWrite])
    for (const write of learningPathWrites) expect(write.revalidate).toBe('/learningpaths')
  })
})
