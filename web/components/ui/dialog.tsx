'use client'

import * as RadixDialog from '@radix-ui/react-dialog'
import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
}

function Dialog({ open, onOpenChange, children }: DialogProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
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
}

function DialogContent({ children, className = '' }: DialogContentProps) {
  return (
    <RadixDialog.Portal>
      <DialogOverlay />
      <RadixDialog.Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2',
          'w-[calc(100%-2rem)] max-w-md max-h-[85vh] overflow-y-auto',
          'rounded-2xl border border-border bg-card shadow-elevated p-5',
          'focus:outline-none',
          'data-[state=open]:animate-in data-[state=closed]:animate-out',
          'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
          'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
          'data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%]',
          'data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]',
          className
        )}
      >
        {children}
      </RadixDialog.Content>
    </RadixDialog.Portal>
  )
}

function DialogHeader({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={cn('mb-4 flex items-center justify-between', className)}>{children}</div>
}

function DialogTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <RadixDialog.Title className={cn('text-base font-semibold text-fg', className)}>
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

function DialogClose({
  children,
  className = '',
  'aria-label': ariaLabel
}: {
  children: ReactNode
  className?: string
  'aria-label'?: string
}) {
  return (
    <RadixDialog.Close
      aria-label={ariaLabel}
      className={cn(
        'rounded-full p-1 text-muted transition-colors hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        className
      )}
    >
      {children}
    </RadixDialog.Close>
  )
}

export { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogClose }
