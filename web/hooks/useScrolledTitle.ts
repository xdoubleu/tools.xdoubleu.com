import { useEffect, useState } from 'react'

/**
 * The page's visible `<h1>` text once it has scrolled under the top bar, else null.
 * `barHeight` is the bar's height in px; `key` re-measures on navigation.
 */
export function useScrolledTitle(key: string, barHeight = 56): string | null {
  const [title, setTitle] = useState<string | null>(null)

  useEffect(() => {
    const measure = () => {
      const h1 = document.querySelector('main h1:not(.sr-only)')
      const scrolledPast = h1 !== null && h1.getBoundingClientRect().bottom < barHeight
      // Stryker disable next-line OptionalChaining: an element always has textContent.
      setTitle(scrolledPast ? (h1.textContent?.trim() ?? null) : null)
    }
    measure()
    window.addEventListener('scroll', measure, { passive: true })
    return () => window.removeEventListener('scroll', measure)
  }, [key, barHeight])

  return title
}
