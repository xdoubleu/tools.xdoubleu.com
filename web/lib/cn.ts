import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merge class names, resolving conflicting Tailwind utilities so that classes
 * passed via a component's `className` prop reliably override the component's
 * own defaults (e.g. `<Button className="w-full" />`).
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
