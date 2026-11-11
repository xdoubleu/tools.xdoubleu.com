import { Suspense, useEffect } from 'react'
import { Outlet, ScrollRestoration } from 'react-router'
import AppShell from '@/components/AppShell'
import { PageLoading } from '@/components/ui/PageLoading'
import { usePathname } from '@/hooks/navigation'
import { artworkApp } from '@/lib/apps'
import { WebVitals } from './WebVitals'

function useAppIcon(pathname: string) {
  useEffect(() => {
    const app = pathname.split('/')[1] ?? ''
    const own = artworkApp(app) !== undefined
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
