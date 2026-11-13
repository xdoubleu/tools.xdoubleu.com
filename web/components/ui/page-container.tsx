import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

const frame = 'max-w-7xl'

const sizes = {
  /** Reading and collection pages. */
  default: frame,
  /** Tables, boards, week grids and full-height dashboards. */
  wide: '',
  /** Settings and single-column forms: the content under the `PageHeader` is capped. */
  narrow: cn(frame, '[&>:not(header)]:max-w-xl'),
  /** Longer create/edit forms. */
  form: cn(frame, '[&>:not(header)]:max-w-2xl'),
  /** Pages outside the shell's flow: sign-in, consent and route errors. */
  centered: 'mx-auto max-w-xl'
} as const

interface PageContainerProps extends HTMLAttributes<HTMLDivElement> {
  size?: keyof typeof sizes
}

/** Every size but `centered` starts at the shell's left edge, so titles line up across pages. */
export function PageContainer({ size = 'default', className, ...props }: PageContainerProps) {
  return <div className={cn('w-full', sizes[size], className)} {...props} />
}
