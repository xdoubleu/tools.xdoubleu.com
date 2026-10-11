import { create } from '@bufbuild/protobuf'
import {
  BookReadingStateDataSchema,
  PhysicalChapterSchema,
  PhysicalCopySchema
} from '@/lib/gen/books/v1/library_pb'
import {
  chapterAt,
  chapterValue,
  continueHint,
  copyDraft,
  savableDraft
} from '@/lib/books/physicalCopy'

describe('physicalCopy', () => {
  it('drafts the stored copy, defaulting the text start to page 1', () => {
    expect(copyDraft(undefined)).toEqual({ pageCount: 0, textStartPage: 1, anchors: [] })
    const copy = create(PhysicalCopySchema, {
      pageCount: 300,
      textStartPage: 0,
      anchors: [{ page: 87, position: { href: 'ch2.xhtml', offset: 9 } }, { page: 5 }]
    })
    expect(copyDraft(copy)).toEqual({
      pageCount: 300,
      textStartPage: 1,
      anchors: [
        { page: 87, href: 'ch2.xhtml', offset: 9 },
        { page: 5, href: '', offset: 0 }
      ]
    })
  })

  it('saves only anchors with a page and a position', () => {
    expect(
      savableDraft({
        pageCount: 1,
        textStartPage: 1,
        anchors: [
          { page: 0, href: 'a', offset: 0 },
          { page: 3, href: '', offset: 0 },
          { page: 4, href: 'b', offset: 2 }
        ]
      }).anchors
    ).toEqual([{ page: 4, href: 'b', offset: 2 }])
  })

  it('finds a chapter by its select value', () => {
    const chapters = [
      create(PhysicalChapterSchema, { title: 'One', position: { href: 'a', offset: 0 } }),
      create(PhysicalChapterSchema, { title: 'Two', position: { href: 'a', offset: 40 } })
    ]
    expect(chapterAt(chapters, chapterValue('a', 40))?.title).toBe('Two')
    expect(chapterAt(chapters, 'b#0')).toBeUndefined()
  })

  it('hints at the page only when the newest position is digital', () => {
    const state = (source: string, physicalPage: number) =>
      create(BookReadingStateDataSchema, { source, physicalPage, physicalExcerpt: 'Hi.' })
    expect(continueHint(state('kobo', 12))).toEqual({ page: 12, excerpt: 'Hi.' })
    expect(continueHint(state('physical', 12))).toBeNull()
    expect(continueHint(state('web', 0))).toBeNull()
    expect(continueHint(undefined)).toBeNull()
  })
})
