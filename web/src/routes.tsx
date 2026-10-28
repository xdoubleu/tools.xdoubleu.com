import { wrapCreateBrowserRouter } from '@sentry/react'
import { createBrowserRouter, redirect, type RouteObject } from 'react-router'
import { PageLoading } from '@/components/ui/PageLoading'
import RootLayout from './RootLayout'
import { NotFound, RouteError } from './RouteStates'

type PageModule = { default: React.ComponentType }

const pages = import.meta.glob<PageModule>('../app/**/page.tsx')

// ../app/books/[id]/read/page.tsx -> books/:id/read ('' for the home page)
export function routePath(file: string): string {
  return file
    .replace(/^\.\.\/app\/?/, '')
    .replace(/\/?page\.tsx$/, '')
    .replace(/\[([^\]]+)\]/g, ':$1')
}

export function pageRoutes(modules: Record<string, () => Promise<PageModule>>): RouteObject[] {
  return Object.entries(modules).map(([file, load]) => ({
    path: routePath(file),
    lazy: async () => ({ Component: (await load()).default })
  }))
}

// Paths with no page of their own.
export const redirectRoutes: RouteObject[] = [
  { path: 'books/feed', loader: () => redirect('/feeds') },
  { path: 'learningpaths', loader: () => redirect('/learningpaths/list') },
  { path: 'recipes', loader: () => redirect('/recipes/list') }
]

export const router = wrapCreateBrowserRouter(createBrowserRouter)([
  {
    Component: RootLayout,
    // Shown while the first page's chunk loads.
    HydrateFallback: PageLoading,
    ErrorBoundary: RouteError,
    children: [...pageRoutes(pages), ...redirectRoutes, { path: '*', Component: NotFound }]
  }
])
