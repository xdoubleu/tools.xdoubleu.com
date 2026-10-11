'use client'

import { Button } from '@/components/ui/button'
import { Collapsible } from '@/components/ui/collapsible'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import type { PhysicalChapter } from '@/lib/gen/books/v1/library_pb'
import {
  chapterAt,
  chapterValue,
  type PhysicalAnchorDraft,
  type PhysicalCopyDraft
} from '@/lib/books/physicalCopy'

interface BookPhysicalCopyFieldsProps {
  draft: PhysicalCopyDraft
  chapters: PhysicalChapter[]
  hasPrintPages: boolean
  onChange: (draft: PhysicalCopyDraft) => void
}

const numberProps = { type: 'number', inputMode: 'numeric', min: 0 } as const

/**
 * The physical copy's page count, where its text starts and chapter anchors,
 * which map its pages to the EPUB. An EPUB with the print edition's page
 * numbers maps them exactly, so only the page count applies.
 */
export default function BookPhysicalCopyFields({
  draft,
  chapters,
  hasPrintPages,
  onChange
}: BookPhysicalCopyFieldsProps) {
  const setAnchor = (i: number, anchor: PhysicalAnchorDraft) =>
    onChange({ ...draft, anchors: draft.anchors.map((a, j) => (j === i ? anchor : a)) })
  const addAnchor = () => {
    const first = chapters[0]?.position
    onChange({
      ...draft,
      anchors: [...draft.anchors, { page: 0, href: first?.href ?? '', offset: first?.offset ?? 0 }]
    })
  }

  return (
    <Collapsible title="Copy details" triggerClassName="text-base">
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <Field label="Pages in your copy" htmlFor="copy-page-count">
            <Input
              id="copy-page-count"
              {...numberProps}
              value={draft.pageCount}
              onChange={(e) => onChange({ ...draft, pageCount: Number(e.target.value) })}
            />
          </Field>
          {!hasPrintPages && (
            <Field label="Text starts on page" htmlFor="copy-text-start">
              <Input
                id="copy-text-start"
                {...numberProps}
                value={draft.textStartPage}
                onChange={(e) => onChange({ ...draft, textStartPage: Number(e.target.value) })}
              />
            </Field>
          )}
        </div>
        {hasPrintPages ? (
          <p className="text-sm text-muted">
            This ebook carries the print edition&apos;s page numbers, so pages map exactly.
          </p>
        ) : (
          <>
            <p className="text-sm text-muted">
              Pin chapters to the pages they start on to line up your copy more closely.
            </p>
            {draft.anchors.map((anchor, i) => (
              <div key={i} className="flex items-end gap-2">
                <Field label="Chapter" htmlFor={`anchor-chapter-${i}`} className="min-w-0 flex-1">
                  <Select
                    id={`anchor-chapter-${i}`}
                    value={chapterValue(anchor.href, anchor.offset)}
                    onChange={(e) => {
                      const pos = chapterAt(chapters, e.target.value)?.position
                      if (pos) setAnchor(i, { ...anchor, href: pos.href, offset: pos.offset })
                    }}
                  >
                    {chapters.map((c) => {
                      const value = chapterValue(c.position?.href ?? '', c.position?.offset ?? 0)
                      return (
                        <option key={value} value={value}>
                          {c.title}
                        </option>
                      )
                    })}
                  </Select>
                </Field>
                <Field label="Page" htmlFor={`anchor-page-${i}`} className="w-24 shrink-0">
                  <Input
                    id={`anchor-page-${i}`}
                    {...numberProps}
                    value={anchor.page}
                    onChange={(e) => setAnchor(i, { ...anchor, page: Number(e.target.value) })}
                  />
                </Field>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Remove chapter anchor"
                  onClick={() =>
                    onChange({ ...draft, anchors: draft.anchors.filter((_, j) => j !== i) })
                  }
                >
                  ✕
                </Button>
              </div>
            ))}
            {chapters.length > 0 && (
              <Button variant="secondary" size="sm" onClick={addAnchor}>
                Add chapter anchor
              </Button>
            )}
          </>
        )}
      </div>
    </Collapsible>
  )
}
