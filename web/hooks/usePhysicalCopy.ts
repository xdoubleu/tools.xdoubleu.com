import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { swrKeys } from '@/lib/swrKeys'
import { updatePhysicalCopyWrite } from '@/lib/books/offlineWrites'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { LibraryService, type GetPhysicalCopyResponse } from '@/lib/gen/books/v1/library_pb'
import type { PhysicalCopyDraft } from '@/lib/books/physicalCopy'

/** The physical copy's details; `bookId` null skips the fetch. */
export function usePhysicalCopy(bookId: string | null) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetPhysicalCopyResponse, Error>(bookId ? swrKeys.physicalCopy(bookId) : null, () =>
    client.getPhysicalCopy({ bookId: bookId! })
  )
}

/** Saves the copy's details through the outbox, so it queues while offline. */
export function useUpdatePhysicalCopy() {
  return async (bookId: string, draft: PhysicalCopyDraft) => {
    await sendWrite(
      enqueueWrite(updatePhysicalCopyWrite, {
        bookId,
        copy: {
          pageCount: draft.pageCount,
          textStartPage: draft.textStartPage,
          anchors: draft.anchors.map((a) => ({
            page: a.page,
            position: { href: a.href, offset: a.offset }
          }))
        }
      })
    )
  }
}
