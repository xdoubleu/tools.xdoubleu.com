'use client'

import Link from 'next/link'
import { mutate } from 'swr'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useAdmin'
import type { AppUser } from '@/lib/gen/admin/v1/admin_pb'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { PageContainer } from '@/components/ui/page-container'
import { swrKeys } from '@/lib/swrKeys'

const APP_NAMES = [
  'games',
  'reading',
  'icsproxy',
  'mealplans',
  'recipes',
  'shoppinglist',
  'todos',
  'watchparty'
]

function UserRow({ user }: { user: AppUser }) {
  const setRole = useSetRole()
  const setAppAccess = useSetAppAccess()

  async function handleRoleChange(role: string) {
    await setRole(user.id, role)
    await mutate(swrKeys.adminUsers)
  }

  async function handleAccessToggle(appName: string) {
    const grant = !user.appAccess.includes(appName)
    await setAppAccess(user.id, appName, grant)
    await mutate(swrKeys.adminUsers)
  }

  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-3 py-2 text-sm text-fg">{user.email}</td>
      <td className="px-3 py-2">
        <Select
          value={user.role}
          onChange={(e) => handleRoleChange(e.target.value)}
          className="h-9 w-auto"
        >
          <option value="user">user</option>
          <option value="admin">admin</option>
        </Select>
      </td>
      {APP_NAMES.map((appName) => {
        const hasAccess = user.appAccess.includes(appName)
        return (
          <td key={appName} className="px-3 py-2 text-center">
            <Button
              size="sm"
              variant={hasAccess ? 'default' : 'secondary'}
              onClick={() => handleAccessToggle(appName)}
            >
              {hasAccess ? 'Revoke' : 'Grant'}
            </Button>
          </td>
        )
      })}
    </tr>
  )
}

export default function AdminUsersClient() {
  const { data, isLoading, error } = useUsers()
  const users = data?.users ?? []

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-danger">Failed to load users.</p>
  }

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">User Management</h1>
        <Button asChild variant="secondary">
          <Link href="/admin/observability">Observability</Link>
        </Button>
      </div>

      <div className="overflow-x-auto rounded-2xl border border-border">
        <table className="w-full text-left">
          <thead className="border-b border-border bg-surface">
            <tr>
              <th className="px-3 py-2 text-sm font-semibold text-subtle">Email</th>
              <th className="px-3 py-2 text-sm font-semibold text-subtle">Role</th>
              {APP_NAMES.map((name) => (
                <th
                  key={name}
                  className="px-3 py-2 text-center text-sm font-semibold capitalize text-subtle"
                >
                  {name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-card">
            {users.length === 0 ? (
              <tr>
                <td
                  colSpan={2 + APP_NAMES.length}
                  className="px-3 py-6 text-center text-sm text-muted"
                >
                  No users found.
                </td>
              </tr>
            ) : (
              users.map((user) => <UserRow key={user.id} user={user} />)
            )}
          </tbody>
        </table>
      </div>
    </PageContainer>
  )
}
