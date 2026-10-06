'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import type { FoliateView } from '@/lib/books/foliate'
import type { ReaderChoiceControl } from '@/lib/books/readerChoice'
import type { ReaderTheme } from '@/lib/books/readerSettings'
import ReaderSettingsSheet from './ReaderSettingsSheet'
import ReaderTocDrawer from './ReaderTocDrawer'

interface ReaderControlsProps {
  view: FoliateView
  currentHref?: string
  theme: ReaderTheme
  onThemeChange: (theme: ReaderTheme) => void
  fontSize: number
  onFontSizeChange: (fontSize: number) => void
  format?: ReaderChoiceControl
}

/** Header controls for an opened book: the contents drawer and reading settings. */
export default function ReaderControls({
  view,
  currentHref,
  theme,
  onThemeChange,
  fontSize,
  onFontSizeChange,
  format
}: ReaderControlsProps) {
  const [tocOpen, setTocOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const toc = view.book?.toc ?? []

  return (
    <>
      {toc.length > 0 && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Contents"
          onClick={() => setTocOpen(true)}
        >
          <span aria-hidden="true" className="text-lg">
            ☰
          </span>
        </Button>
      )}
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label="Reading settings"
        onClick={() => setSettingsOpen(true)}
      >
        <span aria-hidden="true" className="text-sm font-semibold">
          Aa
        </span>
      </Button>

      <ReaderTocDrawer
        open={tocOpen}
        onOpenChange={setTocOpen}
        toc={toc}
        currentHref={currentHref}
        onSelect={(href) => {
          setTocOpen(false)
          void view.goTo(href)
        }}
      />
      <ReaderSettingsSheet
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        theme={theme}
        onThemeChange={onThemeChange}
        fontSize={fontSize}
        onFontSizeChange={onFontSizeChange}
        reflowable={!view.isFixedLayout}
        format={format}
      />
    </>
  )
}
