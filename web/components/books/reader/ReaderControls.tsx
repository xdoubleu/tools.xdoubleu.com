'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import type { FoliateTocItem } from '@/lib/books/foliate'
import type { ReaderTheme } from '@/lib/books/readerSettings'
import ReaderSettingsSheet from './ReaderSettingsSheet'
import ReaderTocDrawer from './ReaderTocDrawer'

interface ReaderControlsProps {
  toc: FoliateTocItem[]
  /** Navigates to a contents entry. */
  onGoTo: (href: string) => void
  reflowable: boolean
  currentHref?: string
  theme: ReaderTheme
  onThemeChange: (theme: ReaderTheme) => void
  fontSize: number
  onFontSizeChange: (fontSize: number) => void
}

/** Header controls for an opened book: the contents drawer and reading settings. */
export default function ReaderControls({
  toc,
  onGoTo,
  reflowable,
  currentHref,
  theme,
  onThemeChange,
  fontSize,
  onFontSizeChange
}: ReaderControlsProps) {
  const [tocOpen, setTocOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

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
          onGoTo(href)
        }}
      />
      <ReaderSettingsSheet
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        theme={theme}
        onThemeChange={onThemeChange}
        fontSize={fontSize}
        onFontSizeChange={onFontSizeChange}
        reflowable={reflowable}
      />
    </>
  )
}
