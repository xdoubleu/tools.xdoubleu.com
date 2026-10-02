import { isMessage } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetLearningPathResponseSchema,
  LearningPathsService,
  ListLearningPathsResponseSchema,
  type LearningPath
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'

const { method } = LearningPathsService
const REVALIDATE = '/learningpaths'

/** Applies `fn` to the cached paths: the list and each path's own response. */
function onPaths(key: unknown, data: unknown, fn: (path: LearningPath) => LearningPath): unknown {
  if (key === swrKeys.learningPaths && isMessage(data, ListLearningPathsResponseSchema)) {
    return { ...data, learningPaths: data.learningPaths.map(fn) }
  }
  if (
    typeof key === 'string' &&
    key.startsWith(`${REVALIDATE}/`) &&
    isMessage(data, GetLearningPathResponseSchema) &&
    data.learningPath
  ) {
    return { ...data, learningPath: fn(data.learningPath) }
  }
  return data
}

export const recordItemProgressWrite = defineOfflineWrite({
  method: method.recordItemProgress,
  apply: (key, data, req) =>
    onPaths(key, data, (path) => ({
      ...path,
      modules: path.modules.map((m) => ({
        ...m,
        // Book-linked items complete from reading progress; the API refuses them.
        items: m.items.map((i) =>
          i.id === req.itemId && i.linkedBookId === undefined
            ? { ...i, completed: req.completed }
            : i
        )
      }))
    })),
  describe: (req) => (req.completed ? 'Complete a learning item' : 'Reopen a learning item'),
  revalidate: REVALIDATE
})

export const setPausedWrite = defineOfflineWrite({
  method: method.setLearningPathPaused,
  apply: (key, data, req) =>
    onPaths(key, data, (path) => (path.id === req.id ? { ...path, paused: req.paused } : path)),
  describe: (req) => (req.paused ? 'Pause a learning path' : 'Resume a learning path'),
  revalidate: REVALIDATE
})

export const learningPathWrites = [recordItemProgressWrite, setPausedWrite]
