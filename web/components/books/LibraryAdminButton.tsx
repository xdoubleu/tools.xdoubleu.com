'use client'

import { useCurrentUser } from '@/hooks/useAuth'
import { PageHeaderLink } from '@/components/ui/page-header'

export default function LibraryAdminButton() {
  const { data: currentUser } = useCurrentUser()
  if (currentUser?.role !== 'admin') return null

  return <PageHeaderLink href="/books/admin">Admin tools</PageHeaderLink>
}
