import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

const sizes = {
  default: 'max-w-6xl',
  narrow: 'max-w-xl'
} as const

interface PageContainerProps extends HTMLAttributes<HTMLDivElement> {
  size?: keyof typeof sizes
}

export function PageContainer({ size = 'default', className, ...props }: PageContainerProps) {
  return <div className={cn('mx-auto w-full', sizes[size], className)} {...props} />
}
