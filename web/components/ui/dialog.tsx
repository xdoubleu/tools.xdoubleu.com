'use client'

import * as RadixDialog from '@radix-ui/react-dialog'
import { type ReactNode } from 'react'
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
  side?: 'center' | 'right'
}

const centerContentClass = [
  'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
  'w-[calc(100%-2rem)] max-w-md max-h-[85vh]',
  'rounded-2xl p-5',
  'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
  'data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%]',
  'data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%]'
]

const rightContentClass = [
  'inset-y-0 right-0',
  'w-[calc(100%-3rem)] max-w-md h-full',
  'rounded-l-2xl p-5',
  'data-[state=closed]:slide-out-to-right',
  'data-[state=open]:slide-in-from-right'
]

function DialogContent({ children, className = '', side = 'center' }: DialogContentProps) {
  return (
    <RadixDialog.Portal>
      {side !== 'right' && <DialogOverlay />}
      <RadixDialog.Content
        onInteractOutside={side === 'right' ? (e) => e.preventDefault() : undefined}
        className={cn(
          'fixed z-50 overflow-y-auto',
          'border border-border bg-card shadow-elevated',
          'focus:outline-none',
          'data-[state=open]:animate-in data-[state=closed]:animate-out',
          'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
          side === 'right' ? rightContentClass : centerContentClass,
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
