import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService, type GetPhysicalCopyResponse } from '@/lib/gen/books/v1/library_pb'
import type { PhysicalCopyDraft } from '@/lib/books/physicalCopy'

/** The physical copy's details; `bookId` null skips the fetch. */
export function usePhysicalCopy(bookId: string | null) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetPhysicalCopyResponse, Error>(bookId ? swrKeys.physicalCopy(bookId) : null, () =>
    client.getPhysicalCopy({ bookId: bookId! })
  )
}

export function useUpdatePhysicalCopy() {
  const client = createServiceClient(LibraryService)
  return async (bookId: string, draft: PhysicalCopyDraft) => {
    await client.updatePhysicalCopy({
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
    await mutate(swrKeys.physicalCopy(bookId))
  }
}
