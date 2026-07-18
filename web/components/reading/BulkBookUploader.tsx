'use client'

import { useState, DragEvent } from 'react'
import { useUploadBookFile } from '@/hooks/useBooks'
import type { UploadBookFileResult } from '@/hooks/useBooks'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'
import { isBookFile, filesFromDataTransfer, MAX_UPLOAD_BYTES } from '@/lib/reading/zipFiles'
import { runPool } from '@/lib/reading/pool'

// ---------------------------------------------------------------------------
// Upload phase state
// ---------------------------------------------------------------------------

type UploadProgress = {
  processed: number
  failed: number
  total: number
  linked: number
  added: number
  errors: string[]
}

type UploadPhase =
  | { kind: 'idle' }
  | { kind: 'uploading'; progress: UploadProgress }
  | { kind: 'done'; progress: UploadProgress }
  | { kind: 'error'; message: string }

// ---------------------------------------------------------------------------
// BulkBookUploader
// ---------------------------------------------------------------------------

export default function BulkBookUploader() {
  const uploadBookFile = useUploadBookFile()
  const [phase, setPhase] = useState<UploadPhase>({ kind: 'idle' })
  const [dragging, setDragging] = useState(false)

  // Number of files to upload concurrently. Modest to avoid saturating the
  // user's upstream and the server's per-file processing.
  const UPLOAD_CONCURRENCY = 4

  async function processFiles(files: File[]) {
    const books = files.filter(isBookFile)
    if (books.length === 0) return

    const progress: UploadProgress = {
      processed: 0,
      failed: 0,
      total: books.length,
      linked: 0,
      added: 0,
      errors: []
    }
    setPhase({ kind: 'uploading', progress: { ...progress } })

    await runPool(books, UPLOAD_CONCURRENCY, async (file) => {
      if (file.size > MAX_UPLOAD_BYTES) {
        progress.failed++
        const limitMB = Math.round(MAX_UPLOAD_BYTES / (1024 * 1024))
        progress.errors = [
          ...progress.errors,
          `${file.name}: file is too large (max ${limitMB} MB)`
        ]
        setPhase({ kind: 'uploading', progress: { ...progress } })
        return
      }
      try {
        const result: UploadBookFileResult = await uploadBookFile(file)
        progress.processed++
        if (result.matchedExisting) {
          progress.linked++
        } else {
          progress.added++
        }
      } catch (err) {
        progress.failed++
        const msg = err instanceof Error ? err.message : 'Upload failed'
        progress.errors = [...progress.errors, `${file.name}: ${msg}`]
      }
      setPhase({ kind: 'uploading', progress: { ...progress } })
    })

    setPhase({ kind: 'done', progress: { ...progress } })
  }

  async function handleDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragging(false)
    const files = await filesFromDataTransfer(e.dataTransfer)
    processFiles(files)
  }

  function handleDragOver(e: DragEvent<HTMLDivElement>) {
    e.preventDefault()
    setDragging(true)
  }

  function handleDragLeave() {
    setDragging(false)
  }

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    processFiles(Array.from(e.target.files ?? []))
    e.target.value = ''
  }

  function handleReset() {
    setPhase({ kind: 'idle' })
  }

  const busy = phase.kind === 'uploading'

  return (
    <div className="space-y-3">
      <div
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        data-testid="drop-zone"
        className={cn(
          'flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed px-6 py-8 transition-colors',
          dragging
            ? 'border-primary bg-primary/5'
            : 'border-border bg-card hover:border-primary/50',
          busy && 'pointer-events-none opacity-60'
        )}
        onClick={() => !busy && document.getElementById('bulk-file-input')?.click()}
      >
        <p className="text-sm text-muted">
          Drop EPUB or PDF files (or a folder) here, or click to browse
        </p>
        <input
          id="bulk-file-input"
          type="file"
          accept=".epub,.pdf"
          multiple
          className="hidden"
          onChange={handleInputChange}
          data-testid="file-input"
        />
        <input
          id="bulk-folder-input"
          type="file"
          className="hidden"
          onChange={handleInputChange}
          data-testid="folder-input"
          // @ts-expect-error webkitdirectory is not in React's InputHTMLAttributes but is supported in all modern browsers
          webkitdirectory=""
        />
      </div>

      {(phase.kind === 'uploading' || phase.kind === 'done') && (
        <UploadProgressDisplay progress={phase.progress} done={phase.kind === 'done'} />
      )}
      {phase.kind === 'error' && <p className="text-sm text-danger">{phase.message}</p>}

      <div className="flex gap-2">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          disabled={busy}
          onClick={() => document.getElementById('bulk-file-input')?.click()}
        >
          Browse files
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          disabled={busy}
          onClick={() => document.getElementById('bulk-folder-input')?.click()}
        >
          Browse folder
        </Button>
        {(phase.kind === 'done' || phase.kind === 'error') && (
          <Button type="button" variant="secondary" size="sm" onClick={handleReset}>
            Import more
          </Button>
        )}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// UploadProgressDisplay
// ---------------------------------------------------------------------------

interface UploadProgressDisplayProps {
  progress: UploadProgress
  done: boolean
}

function UploadProgressDisplay({ progress, done }: UploadProgressDisplayProps) {
  const { processed, failed, total, linked, added, errors } = progress
  const allFailed = done && failed === total

  return (
    <div className="space-y-2 rounded-xl border border-border bg-card p-3">
      <div className="flex items-center justify-between text-sm">
        <span className="text-fg">
          {processed} / {total} uploaded
          {failed > 0 && <span className="ml-1 text-danger">({failed} failed)</span>}
        </span>
        {!done && <span className="text-xs text-muted">Uploading…</span>}
        {done && !allFailed && <span className="text-xs text-success">Done</span>}
        {done && allFailed && <span className="text-xs text-danger">Failed</span>}
      </div>

      {done && processed > 0 && (
        <p className="text-xs text-muted">
          {linked > 0 && `${linked} linked to existing`}
          {linked > 0 && added > 0 && ' — '}
          {added > 0 && `${added} added as new`}
        </p>
      )}

      {errors.length > 0 && (
        <ul className="space-y-0.5">
          {errors.map((e, i) => (
            <li key={i} className="truncate text-xs text-danger">
              {e}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
