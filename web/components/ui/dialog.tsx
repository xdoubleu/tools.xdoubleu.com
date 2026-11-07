import * as RadixDialog from '@radix-ui/react-dialog'
import {
  type CSSProperties,
  type PointerEvent,
  type ReactNode,
  createContext,
  useContext,
  useRef
} from 'react'
import { Button } from '@/components/ui/button'
import { dragVelocity, shouldDismissSheet } from '@/components/ui/sheetDrag'
import { useKeyboardInset } from '@/hooks/useKeyboardInset'
import { cn } from '@/lib/cn'

const CloseContext = createContext<(open: boolean) => void>(() => {})

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  modal?: boolean
  children: ReactNode
}

function Dialog({ open, onOpenChange, modal = true, children }: DialogProps) {
  return (
    <CloseContext.Provider value={onOpenChange}>
      <RadixDialog.Root open={open} onOpenChange={onOpenChange} modal={modal}>
        {children}
      </RadixDialog.Root>
    </CloseContext.Provider>
  )
}

function DialogOverlay() {
  return (
    <RadixDialog.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm data-[state=open]:animate-fade-in" />
  )
}

interface DialogContentProps {
  children: ReactNode
  className?: string
  /** `sheet` (default) docks to the bottom on phones and centers from `sm` up; `center` opts out. */
  side?: 'center' | 'right' | 'fullscreen' | 'sheet'
}

const centerContentClass = [
  'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
  'w-[calc(100%-2rem)] max-w-md max-h-[85dvh]',
  'rounded-2xl p-5',
  'data-[state=open]:animate-pop-in'
]

const rightContentClass = [
  'inset-y-0 right-0',
  'w-[calc(100%-3rem)] max-w-md h-full',
  'rounded-l-2xl p-5',
  'pt-[calc(1.25rem+var(--inset-top))] pb-[calc(1.25rem+env(safe-area-inset-bottom))]',
  'pr-[calc(1.25rem+env(safe-area-inset-right))]',
  'data-[state=open]:animate-panel-in'
]

// Full-screen on mobile (no floating popup box), centered modal from `sm` up.
const fullscreenContentClass = [
  'inset-0 h-full w-full rounded-none p-0',
  'pt-[var(--inset-top)] pb-[env(safe-area-inset-bottom)]',
  'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:h-auto sm:w-[calc(100%-2rem)]',
  'sm:max-w-2xl sm:max-h-[85dvh] sm:-translate-x-1/2 sm:-translate-y-1/2',
  'sm:rounded-2xl sm:p-5',
  'data-[state=open]:animate-pop-in'
]

// Bottom sheet within thumb reach on phones, riding above the on-screen
// keyboard (`--keyboard-inset`); centered modal from `sm` up.
const sheetContentClass = [
  'inset-x-0 bottom-(--keyboard-inset) w-full rounded-t-2xl p-5',
  'max-h-[min(85dvh,calc(100dvh-var(--keyboard-inset)-var(--inset-top)-1rem))]',
  'pb-[calc(1.25rem+max(0px,env(safe-area-inset-bottom)-var(--keyboard-inset)))]',
  'bg-glass backdrop-blur-xl backdrop-saturate-150 sm:bg-card sm:backdrop-blur-none',
  'motion-safe:data-[state=open]:animate-sheet-in sm:motion-safe:data-[state=open]:animate-pop-in',
  'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:bottom-auto sm:w-[calc(100%-2rem)] sm:max-w-md',
  'sm:max-h-[85dvh]',
  'sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:pb-5'
]

const sideContentClass = {
  center: centerContentClass,
  right: rightContentClass,
  fullscreen: fullscreenContentClass,
  sheet: sheetContentClass
}

function DialogContent({ children, className = '', side = 'sheet' }: DialogContentProps) {
  return (
    <RadixDialog.Portal>
      {side !== 'right' && <DialogOverlay />}
      {side === 'sheet' ? (
        <SheetContent className={className}>{children}</SheetContent>
      ) : (
        <ContentPanel side={side} className={className}>
          {children}
        </ContentPanel>
      )}
    </RadixDialog.Portal>
  )
}

// Portal children mount only while open, so the keyboard listener does too.
function SheetContent({ children, className }: { children: ReactNode; className: string }) {
  const style: CSSProperties & { '--keyboard-inset': string } = {
    '--keyboard-inset': `${useKeyboardInset()}px`
  }
  return (
    <ContentPanel side="sheet" className={className} style={style}>
      <SheetHandle />
      {children}
    </ContentPanel>
  )
}

// Phone-only drag handle: pull the sheet down; a fling or long pull closes it.
function SheetHandle() {
  const onOpenChange = useContext(CloseContext)
  const drag = useRef<{ y: number; lastY: number; lastT: number; v: number } | null>(null)
  const panel = (e: PointerEvent<HTMLDivElement>) =>
    e.currentTarget.closest<HTMLElement>('[data-sheet]')

  const onDown = (e: PointerEvent<HTMLDivElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    drag.current = { y: e.clientY, lastY: e.clientY, lastT: e.timeStamp, v: 0 }
    const el = panel(e)
    if (el) el.style.transition = 'none'
  }
  const onMove = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    const el = panel(e)
    if (!d || !el) return
    d.v = dragVelocity(e.clientY - d.lastY, e.timeStamp - d.lastT)
    d.lastY = e.clientY
    d.lastT = e.timeStamp
    el.style.transform = `translateY(${Math.max(0, e.clientY - d.y)}px)`
  }
  const onUp = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    const el = panel(e)
    drag.current = null
    if (!d || !el) return
    if (shouldDismissSheet(e.clientY - d.y, d.v, el.offsetHeight)) {
      onOpenChange(false)
      return
    }
    el.style.transition = 'transform var(--duration-settle) var(--ease-spring)'
    el.style.transform = ''
  }

  return (
    <div
      aria-hidden="true"
      onPointerDown={onDown}
      onPointerMove={onMove}
      onPointerUp={onUp}
      onPointerCancel={onUp}
      className="-mx-5 -mt-5 mb-1 flex h-7 cursor-grab touch-none justify-center pt-2.5 sm:hidden"
    >
      <span className="h-1 w-9 rounded-xl bg-border" />
    </div>
  )
}

function ContentPanel({
  children,
  className,
  side,
  style
}: {
  children: ReactNode
  className: string
  side: NonNullable<DialogContentProps['side']>
  style?: CSSProperties
}) {
  return (
    <RadixDialog.Content
      onInteractOutside={side === 'right' ? (e) => e.preventDefault() : undefined}
      style={style}
      data-sheet={side === 'sheet' ? '' : undefined}
      className={cn(
        'fixed z-50 overflow-x-hidden overflow-y-auto',
        'border border-border shadow-elevated',
        side !== 'sheet' && 'bg-card',
        'focus:outline-none',
        sideContentClass[side],
        'motion-reduce:data-[state=open]:animate-fade-in',
        className
      )}
    >
      {children}
    </RadixDialog.Content>
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
        '-m-2 inline-flex size-11 shrink-0 items-center justify-center rounded-xl text-xl leading-none text-muted',
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
