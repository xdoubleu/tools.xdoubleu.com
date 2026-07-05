import type { Metadata } from 'next'
import { PageContainer } from '@/components/ui/page-container'

export const metadata: Metadata = {
  title: 'Todos',
  description: 'Task management',
  appleWebApp: {
    capable: true,
    title: 'Todos',
    statusBarStyle: 'black-translucent'
  }
}

export default function TodosLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col flex-1">
      <PageContainer className="flex-1 px-4 py-6">{children}</PageContainer>
    </div>
  )
}
