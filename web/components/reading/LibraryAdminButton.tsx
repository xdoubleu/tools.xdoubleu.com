'use client'

import Link from 'next/link'
import { useCurrentUser } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'

export default function LibraryAdminButton() {
  const { data: currentUser } = useCurrentUser()
  if (currentUser?.role !== 'admin') return null

  return (
    <Button asChild variant="secondary" size="sm">
      <Link href="/reading/admin">Admin tools</Link>
    </Button>
  )
}
