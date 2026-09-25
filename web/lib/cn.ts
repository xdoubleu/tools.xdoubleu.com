import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Merges class names so a `className` prop's Tailwind utilities override defaults. */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
