'use client'

import * as RadixDialog from '@radix-ui/react-dialog'
import { type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  modal?: boolean
  children: ReactNode
}

function Dialog({ open, onOpenChange, modal = true, children }: DialogProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange} modal={modal}>
      {children}
    </RadixDialog.Root>
  )
}

function DialogOverlay() {
  return (
    <RadixDialog.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
  )
}

interface DialogContentProps {
  children: ReactNode
  className?: string
  /** `sheet` is a bottom sheet on phones and a centered dialog from `sm` up. */
  side?: 'center' | 'right' | 'fullscreen' | 'sheet'
}

const centerContentClass = [
  'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
  'w-[calc(100%-2rem)] max-w-md max-h-[85dvh]',
  'rounded-2xl p-5',
  'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
  'data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%]',
  'data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]'
]

const rightContentClass = [
  'inset-y-0 right-0',
  'w-[calc(100%-3rem)] max-w-md h-full',
  'rounded-l-2xl p-5',
  'pt-[calc(1.25rem+env(safe-area-inset-top))] pb-[calc(1.25rem+env(safe-area-inset-bottom))]',
  'pr-[calc(1.25rem+env(safe-area-inset-right))]',
  'data-[state=closed]:slide-out-to-right',
  'data-[state=open]:slide-in-from-right'
]

// Full-screen on mobile (no floating popup box), centered modal from `sm` up.
const fullscreenContentClass = [
  'inset-0 h-full w-full rounded-none p-0',
  'pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)]',
  'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:h-auto sm:w-[calc(100%-2rem)]',
  'sm:max-w-2xl sm:max-h-[85dvh] sm:-translate-x-1/2 sm:-translate-y-1/2',
  'sm:rounded-2xl sm:p-5',
  'data-[state=closed]:sm:zoom-out-95 data-[state=open]:sm:zoom-in-95'
]

// Bottom sheet within thumb reach on phones, centered modal from `sm` up.
const sheetContentClass = [
  'inset-x-0 bottom-0 w-full max-h-[85dvh] rounded-t-2xl p-5',
  'pb-[calc(1.25rem+env(safe-area-inset-bottom))]',
  'data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom',
  'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:bottom-auto sm:w-[calc(100%-2rem)] sm:max-w-md',
  'sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:pb-5'
]

const sideContentClass = {
  center: centerContentClass,
  right: rightContentClass,
  fullscreen: fullscreenContentClass,
  sheet: sheetContentClass
}

function DialogContent({ children, className = '', side = 'center' }: DialogContentProps) {
  return (
    <RadixDialog.Portal>
      {side !== 'right' && <DialogOverlay />}
      <RadixDialog.Content
        onInteractOutside={side === 'right' ? (e) => e.preventDefault() : undefined}
        className={cn(
          'fixed z-50 overflow-x-hidden overflow-y-auto',
          'border border-border bg-card shadow-elevated',
          'focus:outline-none',
          'data-[state=open]:animate-in data-[state=closed]:animate-out',
          'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
          sideContentClass[side],
          className
        )}
      >
        {children}
      </RadixDialog.Content>
    </RadixDialog.Portal>
  )
}

function DialogHeader({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('mb-4 flex items-start justify-between gap-3', className)}>{children}</div>
  )
}

function DialogTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <RadixDialog.Title
      className={cn('min-w-0 break-words text-base font-semibold text-fg', className)}
    >
      {children}
    </RadixDialog.Title>
  )
}

function DialogDescription({
  children,
  className = ''
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <RadixDialog.Description className={cn('text-sm text-muted', className)}>
      {children}
    </RadixDialog.Description>
  )
}

/** 44px close control; defaults to a `×` glyph labelled "Close". */
function DialogClose({
  children = '×',
  className = '',
  'aria-label': ariaLabel = 'Close'
}: {
  children?: ReactNode
  className?: string
  'aria-label'?: string
}) {
  return (
    <RadixDialog.Close
      aria-label={ariaLabel}
      className={cn(
        '-m-2 inline-flex size-11 shrink-0 items-center justify-center rounded-full text-xl leading-none text-muted',
        'transition-colors hover:bg-hover hover:text-fg active:bg-hover',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        className
      )}
    >
      {children}
    </RadixDialog.Close>
  )
}

/**
 * Right-aligned action row, pinned to the bottom of a scrolling dialog so its
 * actions stay reachable. Put the confirming action last.
 */
function DialogFooter({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'sticky -bottom-5 z-10 -mx-5 mt-5 -mb-5 flex flex-wrap items-center justify-end gap-2',
        'border-t border-border bg-card px-5 py-3',
        className
      )}
    >
      {children}
    </div>
  )
}

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  /** Say what will happen, especially what cannot be undone. */
  description?: ReactNode
  confirmLabel?: string
  /** Shown on the confirm button while `pending` — a `…`-suffixed present participle. */
  pendingLabel?: string
  cancelLabel?: string
  /** Styles the confirm action as destructive. */
  destructive?: boolean
  pending?: boolean
  /** Blocks confirming while required input in `children` is missing. */
  confirmDisabled?: boolean
  onConfirm: () => void
  children?: ReactNode
}

/**
 * Confirmation prompt for an irreversible action. Prefer this over composing
 * `Dialog` by hand so every confirm step reads and behaves the same.
 */
function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = 'Confirm',
  pendingLabel,
  cancelLabel = 'Cancel',
  destructive = false,
  pending = false,
  confirmDisabled = false,
  onConfirm,
  children
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {description !== undefined && <DialogDescription>{description}</DialogDescription>}
        {children}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={pending}>
            {cancelLabel}
          </Button>
          <Button
            variant={destructive ? 'destructive' : 'default'}
            onClick={onConfirm}
            disabled={pending || confirmDisabled}
          >
            {pending && pendingLabel !== undefined ? pendingLabel : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
  DialogFooter,
  ConfirmDialog
}
export type { ConfirmDialogProps }
