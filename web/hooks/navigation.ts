import { useMemo } from 'react'
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams as useRouterSearchParams
} from 'react-router'

export interface Router {
  push(href: string): void
  replace(href: string): void
  back(): void
}

/**
 * Imperative navigation with a stable identity, safe in effect dependencies.
 * A pushed page opens at the top; replace updates the page in place (e.g. a
 * search query), so it keeps the scroll position.
 */
export function useRouter(): Router {
  const navigate = useNavigate()
  return useMemo(
    () => ({
      push: (href) => void navigate(href),
      replace: (href) => void navigate(href, { replace: true, preventScrollReset: true }),
      back: () => void navigate(-1)
    }),
    [navigate]
  )
}

export function usePathname(): string {
  return useLocation().pathname
}

export function useSearchParams(): URLSearchParams {
  return useRouterSearchParams()[0]
}

/** The route's dynamic segments; the router only matches when all are present. */
export function useRouteParams<T extends Record<string, string>>(): T {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- a matched route always carries its own declared segments
  return useParams() as T
}
