'use client'

import { mutate } from 'swr'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useAdmin'
import type { AppUser } from '@/lib/gen/admin/v1/admin_pb'

const APP_NAMES = ['backlog', 'icsproxy', 'recipes', 'todos', 'watchparty']

function UserRow({ user }: { user: AppUser }) {
  const setRole = useSetRole()
  const setAppAccess = useSetAppAccess()

  async function handleRoleChange(role: string) {
    await setRole(user.id, role)
    await mutate('/admin/users')
  }

  async function handleAccessToggle(appName: string) {
    const grant = !user.appAccess.includes(appName)
    await setAppAccess(user.id, appName, grant)
    await mutate('/admin/users')
  }

  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-3 py-2 text-sm text-fg">{user.email}</td>
      <td className="px-3 py-2">
        <select
          value={user.role}
          onChange={(e) => handleRoleChange(e.target.value)}
          className="rounded border border-border bg-surface px-2 py-1 text-sm text-fg focus:outline-none focus:ring-1 focus:ring-fg"
        >
          <option value="user">user</option>
          <option value="admin">admin</option>
        </select>
      </td>
      {APP_NAMES.map((appName) => {
        const hasAccess = user.appAccess.includes(appName)
        return (
          <td key={appName} className="px-3 py-2 text-center">
            <button
              onClick={() => handleAccessToggle(appName)}
              className={`rounded px-2 py-1 text-xs font-medium ${
                hasAccess
                  ? 'bg-fg text-bg hover:opacity-80'
                  : 'border border-border bg-surface text-muted hover:bg-bg'
              }`}
            >
              {hasAccess ? 'Revoke' : 'Grant'}
            </button>
          </td>
        )
      })}
    </tr>
  )
}

export default function AdminPage() {
  const { data, isLoading, error } = useUsers()
  const users = data?.users ?? []

  if (isLoading) {
    return <p className="py-16 text-center text-sm text-muted">Loading…</p>
  }

  if (error) {
    return <p className="py-16 text-center text-sm text-red-500">Failed to load users.</p>
  }

  return (
    <main className="mx-auto max-w-5xl px-4 py-10">
      <h1 className="mb-6 text-xl font-semibold text-fg">User Management</h1>

      <div className="overflow-x-auto rounded border border-border">
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
          <tbody>
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
    </main>
  )
}
