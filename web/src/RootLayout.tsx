import { Suspense, useEffect } from 'react'
import { Outlet, ScrollRestoration } from 'react-router'
import AppShell from '@/components/AppShell'
import { PageLoading } from '@/components/ui/PageLoading'
import { usePathname } from '@/hooks/navigation'
import { WebVitals } from './WebVitals'

// Apps with their own artwork (public/icons/<app>.svg and -apple.png); the
// icons are read from the DOM when a page is added to the home screen.
const APP_ICONS = new Set([
  'books',
  'feeds',
  'games',
  'mealplans',
  'recipes',
  'shoppinglist',
  'watchparty'
])

function useAppIcon(pathname: string) {
  useEffect(() => {
    const app = pathname.split('/')[1] ?? ''
    const own = APP_ICONS.has(app)
    document
      .getElementById('app-icon')
      ?.setAttribute('href', own ? `/icons/${app}.svg` : '/icon.svg')
    document
      .getElementById('app-apple-icon')
      ?.setAttribute('href', own ? `/icons/${app}-apple.png` : '/apple-icon.png')
  }, [pathname])
}

export default function RootLayout() {
  useAppIcon(usePathname())

  return (
    <>
      <ScrollRestoration />
      <WebVitals />
      <AppShell>
        <Suspense fallback={<PageLoading />}>
          <Outlet />
        </Suspense>
      </AppShell>
    </>
  )
}
