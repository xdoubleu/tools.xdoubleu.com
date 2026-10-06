'use client'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import {
  READER_FONT_SIZE_MAX,
  READER_FONT_SIZE_MIN,
  READER_THEMES,
  stepFontSize,
  type ReaderTheme
} from '@/lib/books/readerSettings'
import { readerChoiceOptions, type ReaderChoiceControl } from '@/lib/books/readerChoice'

interface ReaderSettingsSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  theme: ReaderTheme
  onThemeChange: (theme: ReaderTheme) => void
  fontSize: number
  onFontSizeChange: (fontSize: number) => void
  /** Font size only applies to reflowable books; PDFs are fixed layout. */
  reflowable: boolean
  format?: ReaderChoiceControl
}

export default function ReaderSettingsSheet({
  open,
  onOpenChange,
  theme,
  onThemeChange,
  fontSize,
  onFontSizeChange,
  reflowable,
  format
}: ReaderSettingsSheetProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent side="sheet">
        <DialogHeader>
          <DialogTitle>Reading settings</DialogTitle>
          <DialogClose aria-label="Close reading settings" />
        </DialogHeader>
        <div className="mt-4 space-y-5">
          {format && (
            <div>
              <p className="mb-2 text-xs text-muted">Format</p>
              <SegmentedTabs
                aria-label="Format"
                value={format.value}
                onChange={format.onChange}
                options={readerChoiceOptions(format.original)}
              />
            </div>
          )}
          <div>
            <p className="mb-2 text-xs text-muted">Theme</p>
            <SegmentedTabs
              aria-label="Theme"
              value={theme}
              onChange={onThemeChange}
              options={READER_THEMES}
            />
          </div>
          {reflowable && (
            <div>
              <p className="mb-2 text-xs text-muted">Text size</p>
              <div className="flex items-center gap-3">
                <Button
                  type="button"
                  variant="secondary"
                  size="icon"
                  aria-label="Smaller text"
                  disabled={fontSize <= READER_FONT_SIZE_MIN}
                  onClick={() => onFontSizeChange(stepFontSize(fontSize, -1))}
                >
                  <span aria-hidden="true" className="text-sm">
                    A
                  </span>
                </Button>
                <span className="w-12 text-center text-sm tabular-nums">{fontSize}%</span>
                <Button
                  type="button"
                  variant="secondary"
                  size="icon"
                  aria-label="Larger text"
                  disabled={fontSize >= READER_FONT_SIZE_MAX}
                  onClick={() => onFontSizeChange(stepFontSize(fontSize, 1))}
                >
                  <span aria-hidden="true" className="text-lg">
                    A
                  </span>
                </Button>
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
