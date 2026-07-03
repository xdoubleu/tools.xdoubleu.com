'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useRenameShelf, useDeleteShelf, useRenameTag, useDeleteTag } from '@/hooks/useBooks'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { BUILT_IN_STATUSES, BOOK_STATUSES } from '@/lib/books/bookShelves'
import type { Shelf } from '@/components/books/LibrarySidebar'

interface ManageShelvesTagsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  shelves: Shelf[]
  tags: string[]
}

type RenameState = { type: 'shelf' | 'tag'; name: string; newName: string }
type DeleteShelfState = { name: string; targetName: string }

export default function ManageShelvesTagsDialog({
  open,
  onOpenChange,
  shelves,
  tags
}: ManageShelvesTagsDialogProps) {
  const [renaming, setRenaming] = useState<RenameState | null>(null)
  const [deletingShelf, setDeletingShelf] = useState<DeleteShelfState | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const renameShelf = useRenameShelf()
  const deleteShelf = useDeleteShelf()
  const renameTag = useRenameTag()
  const deleteTag = useDeleteTag()

  const customShelves = shelves.filter((s) => s.id !== 'all' && !BUILT_IN_STATUSES.has(s.id))

  const builtInShelves = shelves.filter((s) => s.id !== 'all' && BUILT_IN_STATUSES.has(s.id))

  const handleRename = async () => {
    if (!renaming || !renaming.newName.trim()) return
    setBusy(true)
    setError(null)
    try {
      if (renaming.type === 'shelf') {
        await renameShelf(renaming.name, renaming.newName.trim())
      } else {
        await renameTag(renaming.name, renaming.newName.trim())
      }
      mutate('/books')
      setRenaming(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Rename failed.')
    } finally {
      setBusy(false)
    }
  }

  const handleDeleteShelf = async () => {
    if (!deletingShelf || !deletingShelf.targetName) return
    setBusy(true)
    setError(null)
    try {
      await deleteShelf(deletingShelf.name, deletingShelf.targetName)
      mutate('/books')
      setDeletingShelf(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed.')
    } finally {
      setBusy(false)
    }
  }

  const handleDeleteTag = async (name: string) => {
    setBusy(true)
    setError(null)
    try {
      await deleteTag(name)
      mutate('/books')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed.')
    } finally {
      setBusy(false)
    }
  }

  // All shelves valid as a reassignment target (built-in + other custom shelves)
  const deleteTargets = [
    ...BOOK_STATUSES,
    ...customShelves
      .filter((s) => s.id !== deletingShelf?.name)
      .map((s) => ({ value: s.id, label: s.label }))
  ]

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Edit shelves & tags</DialogTitle>
          <DialogClose aria-label="Close">x</DialogClose>
        </DialogHeader>

        {error && <p className="mb-3 text-sm text-danger">{error}</p>}

        {/* Built-in shelves — read-only */}
        <section className="mb-4">
          <h3 className="mb-2 text-xs font-semibold text-muted uppercase tracking-wide">
            Built-in shelves
          </h3>
          <div className="flex flex-col gap-1">
            {builtInShelves.map((shelf) => (
              <div
                key={shelf.id}
                className="flex items-center justify-between rounded-xl px-3 py-2 bg-surface text-sm text-subtle"
              >
                <span>{shelf.label}</span>
                <span className="text-xs text-muted">{shelf.count}</span>
              </div>
            ))}
          </div>
        </section>

        {/* Custom shelves — editable */}
        <section className="mb-4">
          <h3 className="mb-2 text-xs font-semibold text-muted uppercase tracking-wide">
            Custom shelves
          </h3>
          {customShelves.length === 0 && (
            <p className="text-sm text-muted">No custom shelves yet.</p>
          )}
          <div className="flex flex-col gap-2">
            {customShelves.map((shelf) => (
              <div key={shelf.id}>
                {renaming?.type === 'shelf' && renaming.name === shelf.id ? (
                  <div className="flex gap-2">
                    <Input
                      value={renaming.newName}
                      onChange={(e) => setRenaming({ ...renaming, newName: e.target.value })}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') void handleRename()
                        if (e.key === 'Escape') setRenaming(null)
                      }}
                      autoFocus
                      className="flex-1 h-8 text-sm"
                    />
                    <Button size="sm" onClick={() => void handleRename()} disabled={busy}>
                      Save
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setRenaming(null)}>
                      Cancel
                    </Button>
                  </div>
                ) : deletingShelf?.name === shelf.id ? (
                  <div className="space-y-2 p-3 rounded-xl border border-danger/30 bg-danger/5">
                    <p className="text-sm">
                      Move {shelf.count} book{shelf.count !== 1 ? 's' : ''} from{' '}
                      <strong>{shelf.label}</strong> to:
                    </p>
                    <Select
                      value={deletingShelf.targetName}
                      onChange={(e) =>
                        setDeletingShelf({ ...deletingShelf, targetName: e.target.value })
                      }
                      className="h-8 text-sm"
                    >
                      {deleteTargets.map(({ value, label }) => (
                        <option key={value} value={value}>
                          {label}
                        </option>
                      ))}
                    </Select>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => void handleDeleteShelf()}
                        disabled={busy || !deletingShelf.targetName}
                      >
                        Delete & move
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => setDeletingShelf(null)}>
                        Cancel
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center justify-between rounded-xl px-3 py-2 bg-surface text-sm">
                    <span>{shelf.label}</span>
                    <div className="flex items-center gap-1">
                      <span className="text-xs text-muted mr-2">{shelf.count}</span>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          setRenaming({ type: 'shelf', name: shelf.id, newName: shelf.label })
                        }
                      >
                        Rename
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          setDeletingShelf({
                            name: shelf.id,
                            targetName: BOOK_STATUSES[0].value
                          })
                        }
                        className="text-danger hover:text-danger"
                      >
                        Delete
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </section>

        {/* Tags — editable */}
        <section>
          <h3 className="mb-2 text-xs font-semibold text-muted uppercase tracking-wide">Tags</h3>
          {tags.length === 0 && <p className="text-sm text-muted">No tags yet.</p>}
          <div className="flex flex-col gap-2">
            {tags.map((tag) => (
              <div key={tag}>
                {renaming?.type === 'tag' && renaming.name === tag ? (
                  <div className="flex gap-2">
                    <Input
                      value={renaming.newName}
                      onChange={(e) => setRenaming({ ...renaming, newName: e.target.value })}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') void handleRename()
                        if (e.key === 'Escape') setRenaming(null)
                      }}
                      autoFocus
                      className="flex-1 h-8 text-sm"
                    />
                    <Button size="sm" onClick={() => void handleRename()} disabled={busy}>
                      Save
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setRenaming(null)}>
                      Cancel
                    </Button>
                  </div>
                ) : (
                  <div className="flex items-center justify-between rounded-xl px-3 py-2 bg-surface text-sm">
                    <span>{tag}</span>
                    <div className="flex gap-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => setRenaming({ type: 'tag', name: tag, newName: tag })}
                      >
                        Rename
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => void handleDeleteTag(tag)}
                        disabled={busy}
                        className="text-danger hover:text-danger"
                      >
                        Delete
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </section>
      </DialogContent>
    </Dialog>
  )
}
